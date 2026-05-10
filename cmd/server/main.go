package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	eventVerification        = "event_verification"
	eventBotAddedToGroup     = "bot_added_to_group_chat"
	eventBotRemovedFromGroup = "bot_removed_from_group_chat"
)

type config struct {
	Port                  string
	SeaTalkAppID          string
	SeaTalkAppSecret      string
	SeaTalkSigningSecret  string
	AdminToken            string
	SpreadsheetID         string
	GoogleCredentialsFile string
	GoogleCredentialsJSON string
	EnableChangeSends     bool
	WatchTab              string
	WatchCell             string
	WatchPollSeconds      int
	ChangeSettleSeconds   int
	ReportTab             string
	ReportRange           string
	GroupIDsRange         string
	CardSpreadsheetID     string
	CardDescriptionRange  string
	CardPendingCell       string
	CardAverageWTCell     string
	CardReportLink        string
	PNGDPI                int
	PNGMaxWidth           int
	Timezone              string
}

type app struct {
	cfg      config
	http     *http.Client
	sheets   *sheets.Service
	seaTalk  *seaTalkClient
	renderer *reportRenderer
	sendMu   sync.Mutex
}

type seaTalkClient struct {
	appID     string
	appSecret string
	http      *http.Client
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type reportRenderer struct {
	cfg       config
	http      *http.Client
	tokenSrc  google.Credentials
	sheetsSvc *sheets.Service
}

type callbackEnvelope struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Timestamp int64           `json:"timestamp"`
	AppID     string          `json:"app_id"`
	Event     json.RawMessage `json:"event"`
}

type verificationEvent struct {
	Challenge string `json:"seatalk_challenge"`
}

type groupEvent struct {
	Group struct {
		GroupID string `json:"group_id"`
		Name    string `json:"group_name"`
	} `json:"group"`
}

func main() {
	_ = godotenv.Load()

	cfg := loadConfig()
	ctx := context.Background()

	creds, err := loadGoogleCredentials(ctx, cfg)
	if err != nil {
		log.Fatalf("load Google credentials: %v", err)
	}
	sheetsSvc, err := sheets.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		log.Fatalf("create sheets service: %v", err)
	}

	a := &app{
		cfg:    cfg,
		http:   http.DefaultClient,
		sheets: sheetsSvc,
		seaTalk: &seaTalkClient{
			appID:     cfg.SeaTalkAppID,
			appSecret: cfg.SeaTalkAppSecret,
			http:      http.DefaultClient,
		},
		renderer: &reportRenderer{
			cfg:       cfg,
			http:      http.DefaultClient,
			tokenSrc:  *creds,
			sheetsSvc: sheetsSvc,
		},
	}

	if cfg.EnableChangeSends {
		go a.watchCellChanges(ctx)
	}
	go a.runDailyGroupSync(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("POST /seatalk/callback", a.handleSeaTalkCallback)
	mux.HandleFunc("POST /admin/test-report", a.handleAdminTestReport)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func loadConfig() config {
	return config{
		Port:                  env("PORT", "8080"),
		SeaTalkAppID:          os.Getenv("SEATALK_APP_ID"),
		SeaTalkAppSecret:      os.Getenv("SEATALK_APP_SECRET"),
		SeaTalkSigningSecret:  os.Getenv("SEATALK_SIGNING_SECRET"),
		AdminToken:            os.Getenv("ADMIN_TOKEN"),
		SpreadsheetID:         env("SPREADSHEET_ID", "1_voFSQBXWh5G5IwBZnt19FE1ro9PpHGOGxtlJscnuzA"),
		GoogleCredentialsFile: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		GoogleCredentialsJSON: os.Getenv("GOOGLE_CREDENTIALS_JSON"),
		EnableChangeSends:     envBool("ENABLE_CHANGE_SENDS", true),
		WatchTab:              env("WATCH_TAB", "BAU Backlogs Summary"),
		WatchCell:             env("WATCH_CELL", "F8"),
		WatchPollSeconds:      envInt("WATCH_POLL_SECONDS", 5),
		ChangeSettleSeconds:   envInt("CHANGE_SETTLE_SECONDS", 5),
		ReportTab:             env("REPORT_TAB", "BAU Backlogs Summary"),
		ReportRange:           env("REPORT_RANGE", "C2:R62"),
		GroupIDsRange:         env("GROUP_IDS_RANGE", "BAU Backlogs Summary!A2:A"),
		CardSpreadsheetID:     env("CARD_SPREADSHEET_ID", "1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0"),
		CardDescriptionRange:  env("CARD_DESCRIPTION_RANGE", "'SOC 5 - Pending LH Tab New'!R17:R21"),
		CardPendingCell:       env("CARD_PENDING_CELL", "'SOC 5 - Pending LH Tab New'!Q12"),
		CardAverageWTCell:     env("CARD_AVERAGE_WT_CELL", "'SOC 5 - Pending LH Tab New'!AE14"),
		CardReportLink:        env("CARD_REPORT_LINK", "https://docs.google.com/spreadsheets/d/1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0/edit?gid=1248015344#gid=1248015344"),
		PNGDPI:                envInt("PNG_DPI", 300),
		PNGMaxWidth:           envInt("PNG_MAX_WIDTH", 2400),
		Timezone:              env("TIMEZONE", "Asia/Manila"),
	}
}

func loadGoogleCredentials(ctx context.Context, cfg config) (*google.Credentials, error) {
	scopes := []string{
		sheets.SpreadsheetsScope,
		"https://www.googleapis.com/auth/drive.readonly",
	}
	if cfg.GoogleCredentialsJSON != "" {
		return google.CredentialsFromJSON(ctx, []byte(cfg.GoogleCredentialsJSON), scopes...)
	}
	if cfg.GoogleCredentialsFile == "" {
		return nil, errors.New("GOOGLE_APPLICATION_CREDENTIALS or GOOGLE_CREDENTIALS_JSON is required")
	}
	data, err := os.ReadFile(cfg.GoogleCredentialsFile)
	if err != nil {
		return nil, err
	}
	return google.CredentialsFromJSON(ctx, data, scopes...)
}

func (a *app) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *app) handleAdminTestReport(w http.ResponseWriter, r *http.Request) {
	if a.cfg.AdminToken == "" {
		http.Error(w, "admin endpoint disabled", http.StatusNotFound)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+a.cfg.AdminToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := a.sendReport(r.Context(), "manual test"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("queued"))
}

func (a *app) handleSeaTalkCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if a.cfg.SeaTalkSigningSecret != "" && !validSeaTalkSignature(body, a.cfg.SeaTalkSigningSecret, r.Header.Get("Signature")) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var envelope callbackEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	switch envelope.EventType {
	case eventVerification:
		var event verificationEvent
		if err := json.Unmarshal(envelope.Event, &event); err != nil {
			http.Error(w, "invalid verification event", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"seatalk_challenge": event.Challenge})
	case eventBotAddedToGroup:
		var event groupEvent
		if err := json.Unmarshal(envelope.Event, &event); err == nil && event.Group.GroupID != "" {
			go func() {
				if err := a.addGroupID(context.Background(), event.Group.GroupID); err != nil {
					log.Printf("add group %s: %v", event.Group.GroupID, err)
				}
			}()
		}
		w.WriteHeader(http.StatusOK)
	case eventBotRemovedFromGroup:
		var event groupEvent
		if err := json.Unmarshal(envelope.Event, &event); err == nil && event.Group.GroupID != "" {
			go func() {
				if err := a.removeGroupID(context.Background(), event.Group.GroupID); err != nil {
					log.Printf("remove group %s: %v", event.Group.GroupID, err)
				}
			}()
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func validSeaTalkSignature(body []byte, secret, signature string) bool {
	sum := sha256.Sum256(append(append([]byte{}, body...), []byte(secret)...))
	expected := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signature))) == 1
}

func (a *app) watchCellChanges(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(a.cfg.WatchPollSeconds) * time.Second)
	defer ticker.Stop()

	var baseline string
	var initialized bool
	var lastSent string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			value, err := a.readSingleCell(ctx, fmt.Sprintf("%s!%s", quoteSheet(a.cfg.WatchTab), a.cfg.WatchCell))
			if err != nil {
				log.Printf("watch read failed: %v", err)
				continue
			}
			if !initialized {
				baseline = value
				initialized = true
				log.Printf("watch baseline set to %q", baseline)
				continue
			}
			if value != baseline && value != lastSent {
				changedTo := value
				lastSent = value
				baseline = value
				go func() {
					time.Sleep(time.Duration(a.cfg.ChangeSettleSeconds) * time.Second)
					if err := a.sendReport(context.Background(), "watch cell changed to "+changedTo); err != nil {
						log.Printf("change-triggered send failed: %v", err)
					}
				}()
			}
		}
	}
}

func (a *app) runDailyGroupSync(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.normalizeGroupIDs(ctx); err != nil {
				log.Printf("daily group sync failed: %v", err)
			}
		}
	}
}

func (a *app) sendReport(ctx context.Context, reason string) error {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()

	groups, err := a.groupIDs(ctx)
	if err != nil {
		return fmt.Errorf("read group ids: %w", err)
	}
	if len(groups) == 0 {
		return errors.New("no group ids configured")
	}
	card, err := a.interactiveReportCard(ctx)
	if err != nil {
		return fmt.Errorf("build interactive card: %w", err)
	}
	png, err := a.renderer.RenderPNG(ctx)
	if err != nil {
		return fmt.Errorf("render report: %w", err)
	}
	card.ImageContent = base64.StdEncoding.EncodeToString(png)

	var errs []string
	for _, groupID := range groups {
		if err := a.seaTalk.SendInteractiveUpdate(ctx, groupID, card); err != nil {
			errs = append(errs, fmt.Sprintf("%s interactive: %v", groupID, err))
		}
		time.Sleep(60 * time.Millisecond)
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

type interactiveCard struct {
	Title        string
	Description  string
	ReportLink   string
	ImageContent string
}

func (a *app) interactiveReportCard(ctx context.Context) (interactiveCard, error) {
	spreadsheetID := a.cfg.CardSpreadsheetID
	if spreadsheetID == "" {
		spreadsheetID = a.cfg.SpreadsheetID
	}

	resp, err := a.sheets.Spreadsheets.Values.BatchGet(spreadsheetID).
		Ranges(a.cfg.CardDescriptionRange).
		Ranges(a.cfg.CardPendingCell).
		Ranges(a.cfg.CardAverageWTCell).
		Context(ctx).
		Do()
	if err != nil {
		return interactiveCard{}, err
	}

	var descriptionLines []string
	pending := ""
	averageWT := ""
	if len(resp.ValueRanges) > 0 {
		descriptionLines = flattenValues(resp.ValueRanges[0].Values)
	}
	if len(resp.ValueRanges) > 1 {
		pending = firstValue(resp.ValueRanges[1].Values)
	}
	if len(resp.ValueRanges) > 2 {
		averageWT = firstValue(resp.ValueRanges[2].Values)
	}

	parts := []string{`<mention-tag target="seatalk://user?id=0"/>`}
	parts = append(parts, descriptionLines...)
	parts = append(parts,
		"**Followup Request:**",
		fmt.Sprintf("- Pending: %s", pending),
		fmt.Sprintf("- Ave. WT: %s", averageWT),
	)

	loc, err := time.LoadLocation(a.cfg.Timezone)
	if err != nil {
		loc = time.Local
	}
	return interactiveCard{
		Title:       "Outbound Pending for Dispatch as of " + time.Now().In(loc).Format("3:04PM Jan-02"),
		Description: strings.Join(nonEmpty(parts), "\n"),
		ReportLink:  a.cfg.CardReportLink,
	}, nil
}

func flattenValues(values [][]any) []string {
	var out []string
	for _, row := range values {
		for _, cell := range row {
			value := strings.TrimSpace(fmt.Sprint(cell))
			if value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

func firstValue(values [][]any) string {
	flattened := flattenValues(values)
	if len(flattened) == 0 {
		return ""
	}
	return flattened[0]
}

func nonEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func (s *seaTalkClient) tokenFor(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && time.Now().Before(s.expiresAt.Add(-5*time.Minute)) {
		return s.token, nil
	}
	body := map[string]string{"app_id": s.appID, "app_secret": s.appSecret}
	var resp struct {
		Code           int    `json:"code"`
		AppAccessToken string `json:"app_access_token"`
		Expire         int64  `json:"expire"`
	}
	if err := s.postJSON(ctx, "https://openapi.seatalk.io/auth/app_access_token", "", body, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 || resp.AppAccessToken == "" {
		return "", fmt.Errorf("SeaTalk token response code=%d", resp.Code)
	}
	s.token = resp.AppAccessToken
	s.expiresAt = time.Unix(resp.Expire, 0)
	return s.token, nil
}

func (s *seaTalkClient) SendInteractiveUpdate(ctx context.Context, groupID string, card interactiveCard) error {
	token, err := s.tokenFor(ctx)
	if err != nil {
		return err
	}
	elements := []map[string]any{
		{"element_type": "title", "title": map[string]string{"text": card.Title}},
		{"element_type": "description", "description": map[string]any{
			"format": 1,
			"text":   card.Description,
		}},
	}
	if card.ImageContent != "" {
		elements = append(elements, map[string]any{
			"element_type": "image",
			"image": map[string]string{
				"content": card.ImageContent,
			},
		})
	}
	elements = append(elements, map[string]any{"element_type": "button", "button": map[string]any{
		"button_type": "redirect",
		"text":        "Open report",
		"mobile_link": map[string]string{
			"type": "web",
			"path": card.ReportLink,
		},
		"desktop_link": map[string]string{
			"type": "web",
			"path": card.ReportLink,
		},
	}})

	msg := map[string]any{
		"group_id": groupID,
		"message": map[string]any{
			"tag": "interactive_message",
			"interactive_message": map[string]any{
				"elements": elements,
			},
		},
	}
	return s.postSeaTalkMessage(ctx, token, msg)
}

func (s *seaTalkClient) postSeaTalkMessage(ctx context.Context, token string, payload any) error {
	var resp struct {
		Code      int    `json:"code"`
		MessageID string `json:"message_id"`
	}
	if err := s.postJSON(ctx, "https://openapi.seatalk.io/messaging/v2/group_chat", token, payload, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("SeaTalk message response code=%d", resp.Code)
	}
	return nil
}

func (s *seaTalkClient) postJSON(ctx context.Context, endpoint, token string, payload any, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("POST %s status=%d body=%s", endpoint, resp.StatusCode, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return err
		}
	}
	return nil
}

func (r *reportRenderer) RenderPNG(ctx context.Context) ([]byte, error) {
	dir, err := os.MkdirTemp("", "backlogs-report-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	pdfPath := filepath.Join(dir, "report.pdf")
	pngBase := filepath.Join(dir, "report")
	pngPath := filepath.Join(dir, "report-1.png")
	finalPath := filepath.Join(dir, "report-final.png")

	pdf, err := r.exportPDF(ctx)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(pdfPath, pdf, 0600); err != nil {
		return nil, err
	}
	if err := run(ctx, "pdftoppm", "-png", "-r", strconv.Itoa(r.cfg.PNGDPI), "-singlefile", pdfPath, pngBase); err != nil {
		return nil, err
	}
	singleFilePath := pngBase + ".png"
	if _, err := os.Stat(singleFilePath); err == nil {
		pngPath = singleFilePath
	}
	if r.cfg.PNGMaxWidth > 0 {
		if err := run(ctx, "magick", pngPath, "-resize", fmt.Sprintf("%dx>", r.cfg.PNGMaxWidth), finalPath); err != nil {
			if err := run(ctx, "convert", pngPath, "-resize", fmt.Sprintf("%dx>", r.cfg.PNGMaxWidth), finalPath); err != nil {
				return nil, err
			}
		}
		return os.ReadFile(finalPath)
	}
	return os.ReadFile(pngPath)
}

func (r *reportRenderer) exportPDF(ctx context.Context) ([]byte, error) {
	gid, err := r.sheetGID(ctx, r.cfg.ReportTab)
	if err != nil {
		return nil, err
	}
	token, err := r.tokenSrc.TokenSource.Token()
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("format", "pdf")
	q.Set("gid", strconv.FormatInt(gid, 10))
	q.Set("range", r.cfg.ReportRange)
	q.Set("portrait", "false")
	q.Set("fitw", "true")
	q.Set("sheetnames", "false")
	q.Set("printtitle", "false")
	q.Set("pagenumbers", "false")
	q.Set("gridlines", "false")
	q.Set("fzr", "false")
	endpoint := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?%s", r.cfg.SpreadsheetID, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("export status=%d body=%s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (r *reportRenderer) sheetGID(ctx context.Context, title string) (int64, error) {
	ss, err := r.sheetsSvc.Spreadsheets.Get(r.cfg.SpreadsheetID).Fields("sheets(properties(sheetId,title))").Context(ctx).Do()
	if err != nil {
		return 0, err
	}
	for _, sh := range ss.Sheets {
		if sh.Properties != nil && sh.Properties.Title == title {
			return sh.Properties.SheetId, nil
		}
	}
	return 0, fmt.Errorf("sheet %q not found", title)
}

func (a *app) readSingleCell(ctx context.Context, a1 string) (string, error) {
	resp, err := a.sheets.Spreadsheets.Values.Get(a.cfg.SpreadsheetID, a1).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	if len(resp.Values) == 0 || len(resp.Values[0]) == 0 {
		return "", nil
	}
	return fmt.Sprint(resp.Values[0][0]), nil
}

func (a *app) groupIDs(ctx context.Context) ([]string, error) {
	resp, err := a.sheets.Spreadsheets.Values.Get(a.cfg.SpreadsheetID, a.cfg.GroupIDsRange).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, row := range resp.Values {
		if len(row) == 0 {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(row[0]))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (a *app) addGroupID(ctx context.Context, groupID string) error {
	ids, err := a.groupIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == groupID {
			return nil
		}
	}
	ids = append(ids, groupID)
	sort.Strings(ids)
	return a.writeGroupIDs(ctx, ids)
}

func (a *app) removeGroupID(ctx context.Context, groupID string) error {
	ids, err := a.groupIDs(ctx)
	if err != nil {
		return err
	}
	filtered := ids[:0]
	for _, id := range ids {
		if id != groupID {
			filtered = append(filtered, id)
		}
	}
	return a.writeGroupIDs(ctx, filtered)
}

func (a *app) normalizeGroupIDs(ctx context.Context) error {
	ids, err := a.groupIDs(ctx)
	if err != nil {
		return err
	}
	return a.writeGroupIDs(ctx, ids)
}

func (a *app) writeGroupIDs(ctx context.Context, ids []string) error {
	values := make([][]any, len(ids))
	for i, id := range ids {
		values[i] = []any{id}
	}
	_, err := a.sheets.Spreadsheets.Values.Clear(a.cfg.SpreadsheetID, a.cfg.GroupIDsRange, &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	_, err = a.sheets.Spreadsheets.Values.Update(a.cfg.SpreadsheetID, a.cfg.GroupIDsRange, &sheets.ValueRange{
		Values: values,
	}).ValueInputOption("RAW").Context(ctx).Do()
	return err
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w: %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func quoteSheet(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
