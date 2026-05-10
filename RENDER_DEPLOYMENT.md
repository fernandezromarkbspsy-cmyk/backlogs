# Render Deployment Guide

This guide deploys the SeaTalk bot server to Render using the existing `Dockerfile`.

Render is a good alternative to Azure for this bot because:

- Free tier available for web services
- Built-in Docker support
- Environment variable configuration
- Automatic SSL/HTTPS
- Easy deployment from GitHub

Render Free Tier limitations:
- 512 MB RAM
- 0.1 CPU
- Sleeps after 15 minutes of inactivity (not suitable for the watcher)

**Recommendation**: Use Render Starter plan ($7/month) for Always On support.

## What Will Be Deployed

The container exposes port `8080` and serves:

```text
GET  /healthz
POST /seatalk/callback
POST /admin/test-report
```

The app uses port 8080 by default (no PORT variable needed).

## Prerequisites

You need:

- A Render account (sign up at https://render.com)
- A GitHub repository with your code
- A SeaTalk app with bot capability, event callback, and group message permission enabled
- Google service account credentials with access to these spreadsheets:
  - `1_voFSQBXWh5G5IwBZnt19FE1ro9PpHGOGxtlJscnuzA`
  - `1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0`
- Your Google service account JSON file

## 1. Push Code to GitHub

If your code isn't already on GitHub:

```powershell
git add .
git commit -m "Initial commit"
git branch -M main
git remote add origin https://github.com/YOUR_USERNAME/YOUR_REPO.git
git push -u origin main
```

## 2. Create Render Web Service

1. Go to https://dashboard.render.com
2. Click "New" → "Web Service"
3. Connect your GitHub account and select your repository
4. Configure:

### Build Settings

- **Runtime**: Docker
- **Docker Context**: `/` (root directory)
- **Dockerfile Path**: `Dockerfile`

### Environment Variables

Add these non-secret settings:

| Name | Value |
|------|-------|
| TIMEZONE | Asia/Manila |
| SPREADSHEET_ID | 1_voFSQBXWh5G5IwBZnt19FE1ro9PpHGOGxtlJscnuzA |
| ENABLE_CHANGE_SENDS | true |
| WATCH_TAB | BAU Backlogs Summary |
| WATCH_CELL | F8 |
| WATCH_POLL_SECONDS | 5 |
| CHANGE_SETTLE_SECONDS | 5 |
| REPORT_TAB | BAU Backlogs Summary |
| REPORT_RANGE | C2:R62 |
| GROUP_IDS_RANGE | BAU Backlogs Summary!A2:A |
| CARD_SPREADSHEET_ID | 1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0 |
| CARD_DESCRIPTION_RANGE | 'SOC 5 - Pending LH Tab New'!R17:R21 |
| CARD_PENDING_CELL | 'SOC 5 - Pending LH Tab New'!Q12 |
| CARD_AVERAGE_WT_CELL | 'SOC 5 - Pending LH Tab New'!AE14 |
| CARD_REPORT_LINK | https://docs.google.com/spreadsheets/d/1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0/edit?gid=1248015344#gid=1248015344 |
| PNG_DPI | 300 |
| PNG_MAX_WIDTH | 2400 |

Add these secrets:

| Name | Value |
|------|-------|
| SEATALK_APP_ID | Your SeaTalk app ID |
| SEATALK_APP_SECRET | Your SeaTalk app secret |
| SEATALK_SIGNING_SECRET | Your SeaTalk signing secret |
| ADMIN_TOKEN | Generate with: `-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | % {[char]$_})` |
| GOOGLE_CREDENTIALS_JSON | Paste the entire contents of your Google service account JSON file |

### Advanced Settings

- **Plan**: Starter ($7/month) for Always On, or Free (will sleep when idle)
- **Region**: Choose closest to your users
- **Instance Type**: Standard

5. Click "Create Web Service"

## 3. Wait for Deployment

Render will automatically:
- Build the Docker image
- Deploy the container
- Assign a public URL (e.g., `https://your-app.onrender.com`)

Monitor the deployment in the Render dashboard.

## 4. Verify Health

Once deployed, test the health endpoint:

```powershell
$APP_URL = "https://your-app.onrender.com"
Invoke-WebRequest "$APP_URL/healthz"
```

Expected response:

```text
ok
```

## 5. Configure SeaTalk Callback URL

In the SeaTalk Open Platform app settings, set the event callback URL to:

```text
https://your-app.onrender.com/seatalk/callback
```

When SeaTalk verifies the callback URL, the server responds with the `seatalk_challenge` value.

If verification fails:
- Confirm `SEATALK_SIGNING_SECRET` is correct
- Confirm the app is running at `/healthz`
- Check Render logs
- Make sure the callback URL uses HTTPS (Render provides this automatically)

## 6. Test Manual Report Send

```powershell
$ADMIN_TOKEN = "<same-admin-token-you-set-in-render>"

Invoke-WebRequest `
  -Method POST `
  -Uri "$APP_URL/admin/test-report" `
  -Headers @{ Authorization = "Bearer $ADMIN_TOKEN" }
```

Expected HTTP status:

```text
202 Accepted
```

## 7. View Logs

In the Render dashboard:
1. Go to your web service
2. Click "Logs"
3. View real-time logs or download past logs

## 8. Deploy Updates

When you push changes to GitHub:

1. Render automatically detects the commit
2. Rebuilds the Docker image
3. Deploys the new version

Or trigger a manual deploy:
- Go to your service in Render dashboard
- Click "Manual Deploy" → "Latest commit"

## 9. Keep Service Awake with UptimeRobot (Free Tier)

Render Free tier sleeps after 15 minutes of inactivity. Use UptimeRobot to keep it awake:

1. Go to https://uptimerobot.com
2. Sign up for a free account
3. Click "Add New Monitor"
4. Configure:
   - **Monitor Type**: HTTP(s)
   - **Monitor Friendly Name**: SeaTalk Backlogs Bot
   - **URL (or IP)**: `https://your-app.onrender.com/healthz`
   - **Monitoring Interval**: 5 minutes
   - **Alert Contacts**: Add your email (optional)
5. Click "Create Monitor"

UptimeRobot will ping your `/healthz` endpoint every 5 minutes, keeping the Render service awake.

**Note**: This is not guaranteed to work 100% of the time. For production reliability, use the Starter plan ($7/month) with Always On.

## 10. Common Troubleshooting

### Health Check Fails

The app uses port 8080 by default. Inspect logs in Render dashboard for errors.

### Container Does Not Start

Common causes:
- Missing SeaTalk or Google environment variables
- Invalid `GOOGLE_CREDENTIALS_JSON`
- Docker build errors

Check the "Events" tab in Render for build errors.

### SeaTalk Callback Verification Fails

Verify:
- Callback URL is exactly `https://your-app.onrender.com/seatalk/callback`
- `SEATALK_SIGNING_SECRET` matches the SeaTalk app
- Server logs do not show `invalid signature`
- `/healthz` returns `ok`

### Google Sheets Reads Fail

Verify:
- Service account email has access to both spreadsheets
- `GOOGLE_CREDENTIALS_JSON` contains the full JSON
- `SPREADSHEET_ID` and `CARD_SPREADSHEET_ID` are correct
- Sheet names and ranges match exactly

### Report Rendering Fails

The Docker image installs:
```text
poppler-utils
imagemagick
```

If rendering fails, inspect logs for `pdftoppm`, `magick`, or `convert` errors.

### Watcher Does Not Send

On Render Free tier, the service sleeps after 15 minutes of inactivity. The watcher will not work reliably. Use Starter plan for Always On.

The first watched-cell read is only a baseline and does not send. A send happens only after `WATCH_CELL` changes after startup.

For immediate testing, use:
```text
POST /admin/test-report
```

## 10. Cost Comparison

| Platform | Free Tier | Paid Tier (Always On) |
|----------|-----------|----------------------|
| Render | $0 (sleeps after 15min) | $7/month |
| Azure | Not suitable for watcher | $18-23/month |

Render is more cost-effective for small deployments.

## 11. Clean Up

To delete the Render service:
1. Go to your service in Render dashboard
2. Click "Settings" → "Delete Service"

## Reference

- Render Docker deployment: https://render.com/docs/docker
- Render environment variables: https://render.com/docs/environment-variables
- Render web services: https://render.com/docs/web-services
