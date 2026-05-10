Go SeaTalk bot that watches `BAU Backlogs Summary!F8`. When the value changes, it waits 5 seconds for dependent sheet data to settle, then captures `BAU Backlogs Summary!C2:R62`, renders it as an image, using poppler and image magick and sends an `@all` text update plus the report image to every known group. using seatalk interactive message

## Requirements

- SeaTalk app with bot capability, event callback, and group message permission enabled.
- Google service account with access to spreadsheet `1_voFSQBXWh5G5IwBZnt19FE1ro9PpHGOGxtlJscnuzA`.
- The spreadsheet must have:
  - `BAU Backlogs Summary`
  - group IDs stored in `BAU Backlogs Summary!A2:A`

## Configure

Copy `.env.example` to `.env` and fill in the secrets:

```env
SEATALK_APP_ID=
SEATALK_APP_SECRET=
SEATALK_SIGNING_SECRET=
ADMIN_TOKEN=
GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/google-service-account.json
```

The server loads `.env` automatically for local runs. Environment variables already set outside the file take precedence.

## Change-Triggered Sends

The first read of the watched cell is treated as the baseline and does not send. Later value changes trigger a delayed send.

```env
ENABLE_CHANGE_SENDS=true
WATCH_TAB=BAU Backlogs Summary
WATCH_CELL=F8
WATCH_POLL_SECONDS=5
CHANGE_SETTLE_SECONDS=5
```

Google Sheets limits service-account reads to 60 requests per minute per user. A 5-second poll interval keeps the watcher at about 12 reads per minute before report sends and callback updates.

Image render defaults:

```env
PNG_DPI=300
PNG_MAX_WIDTH=2400
```

Scheduled sends are not implemented. Reports are sent only when the watched cell changes or when the admin test endpoint is called.


## Run

```bash
docker compose up --build
```

Health check:

```text
GET /healthz
```

Manual report test, enabled only when `ADMIN_TOKEN` is set:

```bash
curl -X POST https://your-public-host/admin/test-report \
  -H "Authorization: Bearer your-admin-token"
```

## Deploy On Azure

Use [AZURE_DEPLOYMENT.md](AZURE_DEPLOYMENT.md) to deploy the Docker server to Azure App Service for Linux with Azure Container Registry.

After deployment, use the Azure App Service URL:

```text
https://your-webapp-name.azurewebsites.net/healthz
https://your-webapp-name.azurewebsites.net/seatalk/callback
https://your-webapp-name.azurewebsites.net/admin/test-report
```

## Group ID Handling

When the bot is added to a SeaTalk group, the callback handler stores the `group_id` in `bot_config!A2:A`. When the bot is removed, it removes that ID. A daily sync normalizes the sheet list by sorting and deduplicating known IDs.
