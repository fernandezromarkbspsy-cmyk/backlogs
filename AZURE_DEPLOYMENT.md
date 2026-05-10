# Azure Deployment Guide

This guide deploys the SeaTalk bot server to Azure App Service for Linux using the existing `Dockerfile`.

Azure App Service is the recommended target for this bot because the server has:

- a public HTTPS callback endpoint for SeaTalk
- a long-running Google Sheets watcher
- external Linux tools installed in the container: `poppler-utils` and ImageMagick
- environment-variable based secrets

Use an App Service plan with Always On enabled. Avoid Free or Shared tiers for production because the watcher can be suspended when the app is idle.

## What Will Be Deployed

The container exposes port `8080` and serves:

```text
GET  /healthz
POST /seatalk/callback
POST /admin/test-report
```

Azure must be configured with:

```text
WEBSITES_PORT=8080
PORT=8080
```

`WEBSITES_PORT` tells Azure App Service which container port to route public HTTP traffic to.

## Prerequisites

Install these locally:

- Azure CLI: https://learn.microsoft.com/cli/azure/install-azure-cli
- Docker Desktop: https://docs.docker.com/desktop/
- Git, if deploying from a cloned repo

You also need:

- an Azure subscription (student subscriptions work with this guide)
- a SeaTalk app with bot capability, event callback, and group message permission enabled
- Google service account credentials with access to these spreadsheets:
  - `1_voFSQBXWh5G5IwBZnt19FE1ro9PpHGOGxtlJscnuzA`
  - `1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0`
- your Google service account JSON file

### Azure Student Subscription Notes

Azure student subscriptions (typically $100 free credits) work well with this deployment:

- **App Service Plan B1**: ~$13-18/month - well within student credit limits
- **Container Registry Basic**: ~$0.167/day (~$5/month) - minimal cost
- **Total estimated cost**: ~$18-23/month for always-on bot

Student subscription considerations:
- Credits expire after 12 months
- Some regions may have limited availability
- Monitor credit usage in Azure Portal to avoid unexpected charges
- Delete resources when not in use to conserve credits
- The B1 plan with Always On is recommended for the watcher; Free/Shared tiers will suspend the bot when idle

## Alternative: Azure Portal UI Deployment

If you prefer using the Azure Portal web interface instead of CLI commands, follow these steps:

### A. Create Resource Group

1. Go to https://portal.azure.com
2. Search for "Resource groups" in the search bar
3. Click "Create"
4. Fill in:
   - **Subscription**: Select your student subscription
   - **Resource group**: `rg-seatalk-backlogs`
   - **Region**: Southeast Asia (or your preferred region)
5. Click "Review + create", then "Create"

### B. Create Azure Container Registry

1. In the Azure Portal, search for "Container registries"
2. Click "Create"
3. Fill in:
   - **Resource group**: Select `rg-seatalk-backlogs`
   - **Registry name**: `acrseatalkbacklogs` + random numbers (must be globally unique)
   - **Region**: Same as your resource group
   - **SKU**: Basic
4. Click "Review + create", then "Create"
5. After creation, note the **Login server** (e.g., `acrseatalkbacklogs1234.azurecr.io`)

### C. Enable Admin User for ACR

1. Go to your Container Registry resource
2. Navigate to "Settings" → "Access keys"
3. Toggle "Admin user" to "Enabled"
4. Note the **Username** and **Password** (you'll need these later)

### D. Create App Service Plan

1. Search for "App Service plans" in the portal
2. Click "Create"
3. Fill in:
   - **Resource group**: Select `rg-seatalk-backlogs`
   - **App Service plan**: `asp-seatalk-backlogs`
   - **Region**: Same as your resource group
   - **Operating System**: Linux
   - **SKU**: Basic (B1) - required for Always On
4. Click "Review + create", then "Create"

### E. Build Docker Image in Azure (No Local Docker Required)

If you cannot run Docker locally, use Azure Container Registry Tasks to build directly in the cloud:

**Option 1: Using Azure CLI (Recommended)**

```powershell
# Build and push image directly to ACR without local Docker
az acr build `
  --registry <your-acr-name> `
  --image seatalk-backlogs-bot:v1 `
  --file Dockerfile `
  .
```

This builds the image using Azure's infrastructure and automatically pushes it to your ACR.

**Option 2: Using Azure Portal**

1. Go to your Container Registry in the portal
2. Navigate to "Services" → "Tasks"
3. Click "Create" → "Quick Task"
4. Configure:
   - **Task name**: `build-seatalk-backlogs`
   - **Image source**: Local context (upload your code or use Git)
   - **Dockerfile path**: `Dockerfile`
   - **Image name**: `seatalk-backlogs-bot:v1`
5. Click "Run"

**Option 3: Using GitHub (If your code is on GitHub)**

1. Go to your Container Registry in the portal
2. Navigate to "Services" → "Tasks"
3. Click "Create" → "Quick Task"
4. Select "GitHub" as source
5. Connect your GitHub account and select your repository
6. Configure Dockerfile path and image name
7. Click "Run"

After the build completes, verify the image exists in your ACR:
- Go to your Container Registry → "Repositories"
- You should see `seatalk-backlogs-bot` with tag `v1`

### F. Create Web App

1. Search for "Web App" in the portal
2. Click "Create"
3. Fill in:
   - **Resource group**: Select `rg-seatalk-backlogs`
   - **App Service plan**: Select `asp-seatalk-backlogs`
   - **Name**: `seatalk-backlogs` + random numbers (must be globally unique)
   - **Runtime stack**: Docker
   - **Region**: Same as your resource group
   - **Docker**: Linux
4. Click "Next: Docker" and configure:
   - **Image source**: Azure Container Registry
   - **Registry**: Select your ACR
   - **Image**: `seatalk-backlogs-bot`
   - **Tag**: `v1`
5. Click "Review + create", then "Create"

### G. Configure Web App Settings

1. Go to your Web App resource
2. Navigate to "Settings" → "Configuration"
3. In "Application settings", add these non-secret settings:

| Name | Value |
|------|-------|
| WEBSITES_PORT | 8080 |
| PORT | 8080 |
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

4. In "Application settings", click "New setting" and add these as secrets (check "Secret" checkbox):

| Name | Value |
|------|-------|
| SEATALK_APP_ID | Your SeaTalk app ID |
| SEATALK_APP_SECRET | Your SeaTalk app secret |
| SEATALK_SIGNING_SECRET | Your SeaTalk signing secret |
| ADMIN_TOKEN | Choose a long random token for admin access |
| GOOGLE_CREDENTIALS_JSON | Paste the entire contents of your Google service account JSON file |

5. Click "Save"

### H. Enable Always On

1. In your Web App, navigate to "Settings" → "Configuration"
2. Go to "General settings"
3. Set "Always On" to "On"
4. Click "Save"

### I. Restart and Verify

1. In your Web App, click "Restart" in the top menu
2. Wait for the app to restart
3. Note the default domain URL (e.g., `https://seatalk-backlogs-1234.azurewebsites.net`)
4. Test the health endpoint: `https://<your-app-name>.azurewebsites.net/healthz`
5. Should return "ok"

### J. View Logs (Optional)

1. In your Web App, navigate to "Monitoring" → "Log stream"
2. Or enable container logs:
   - Go to "Settings" → "App Service logs"
   - Set "Application logging (Filesystem)" to "On"
   - Set "Detailed error messages" to "On"
   - Click "Save"
   - Restart the app and view logs in "Log stream"

Continue with steps 9-12 from the CLI guide for SeaTalk configuration and testing.

## 1. Choose Names

Run these commands from PowerShell. Replace values if needed.

```powershell
$RESOURCE_GROUP = "rg-seatalk-backlogs"
$LOCATION = "southeastasia"
$ACR_NAME = "acrseatalkbacklogs$((Get-Random -Minimum 1000 -Maximum 9999))"
$APP_SERVICE_PLAN = "asp-seatalk-backlogs"
$WEBAPP_NAME = "seatalk-backlogs-$((Get-Random -Minimum 1000 -Maximum 9999))"
$IMAGE_NAME = "seatalk-backlogs-bot"
$IMAGE_TAG = "v1"
```

`$WEBAPP_NAME` must be globally unique because it becomes part of the public URL:

```text
https://<WEBAPP_NAME>.azurewebsites.net
```

## 2. Log In To Azure

```powershell
az login
az account show
```

If you have multiple subscriptions:

```powershell
az account set --subscription "<subscription-id-or-name>"
```

## 3. Create Azure Resources

Create a resource group:

```powershell
az group create `
  --name $RESOURCE_GROUP `
  --location $LOCATION
```

Create Azure Container Registry:

```powershell
az acr create `
  --resource-group $RESOURCE_GROUP `
  --name $ACR_NAME `
  --sku Basic `
  --admin-enabled true
```

Create an App Service plan for Linux containers:

```powershell
az appservice plan create `
  --resource-group $RESOURCE_GROUP `
  --name $APP_SERVICE_PLAN `
  --is-linux `
  --sku B1
```

`B1` is a practical minimum because Always On is available on Basic and higher tiers.

## 4. Build And Push The Docker Image

Log in to your Azure Container Registry:

```powershell
az acr login --name $ACR_NAME
```

Get the registry login server:

```powershell
$ACR_LOGIN_SERVER = az acr show `
  --name $ACR_NAME `
  --resource-group $RESOURCE_GROUP `
  --query loginServer `
  --output tsv
```

Build the container:

```powershell
docker build -t "${IMAGE_NAME}:$IMAGE_TAG" .
```

Tag it for ACR:

```powershell
docker tag "${IMAGE_NAME}:$IMAGE_TAG" "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG"
```

Push it:

```powershell
docker push "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG"
```

## 5. Create The Web App

Create the App Service web app from the pushed image:

```powershell
az webapp create `
  --resource-group $RESOURCE_GROUP `
  --plan $APP_SERVICE_PLAN `
  --name $WEBAPP_NAME `
  --deployment-container-image-name "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG"
```

Get the ACR credentials:

```powershell
$ACR_USERNAME = az acr credential show `
  --name $ACR_NAME `
  --query username `
  --output tsv

$ACR_PASSWORD = az acr credential show `
  --name $ACR_NAME `
  --query "passwords[0].value" `
  --output tsv
```

Configure the web app to pull from ACR:

```powershell
az webapp config container set `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME `
  --docker-custom-image-name "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG" `
  --docker-registry-server-url "https://$ACR_LOGIN_SERVER" `
  --docker-registry-server-user $ACR_USERNAME `
  --docker-registry-server-password $ACR_PASSWORD
```

Enable Always On:

```powershell
az webapp config set `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME `
  --always-on true
```

## 6. Prepare Google Credentials

For Azure App Service, use `GOOGLE_CREDENTIALS_JSON` instead of mounting a JSON file.

From the folder containing your service account JSON:

```powershell
$GOOGLE_CREDENTIALS_JSON = Get-Content ".\google-service-account.json" -Raw
```

Do not commit this JSON file to the repo.

## 7. Configure App Settings

Set non-secret runtime settings:

```powershell
az webapp config appsettings set `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME `
  --settings `
    WEBSITES_PORT="8080" `
    PORT="8080" `
    TIMEZONE="Asia/Manila" `
    SPREADSHEET_ID="1_voFSQBXWh5G5IwBZnt19FE1ro9PpHGOGxtlJscnuzA" `
    ENABLE_CHANGE_SENDS="true" `
    WATCH_TAB="BAU Backlogs Summary" `
    WATCH_CELL="F8" `
    WATCH_POLL_SECONDS="5" `
    CHANGE_SETTLE_SECONDS="5" `
    REPORT_TAB="BAU Backlogs Summary" `
    REPORT_RANGE="C2:R62" `
    GROUP_IDS_RANGE="BAU Backlogs Summary!A2:A" `
    CARD_SPREADSHEET_ID="1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0" `
    CARD_DESCRIPTION_RANGE="'SOC 5 - Pending LH Tab New'!R17:R21" `
    CARD_PENDING_CELL="'SOC 5 - Pending LH Tab New'!Q12" `
    CARD_AVERAGE_WT_CELL="'SOC 5 - Pending LH Tab New'!AE14" `
    CARD_REPORT_LINK="https://docs.google.com/spreadsheets/d/1Gtuvntb6wwK1OheNUKnh6SFKI49yNcS1jsCaSuwc5Y0/edit?gid=1248015344#gid=1248015344" `
    PNG_DPI="300" `
    PNG_MAX_WIDTH="2400"
```

Set secrets:

```powershell
az webapp config appsettings set `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME `
  --settings `
    SEATALK_APP_ID="<your-seatalk-app-id>" `
    SEATALK_APP_SECRET="<your-seatalk-app-secret>" `
    SEATALK_SIGNING_SECRET="<your-seatalk-signing-secret>" `
    ADMIN_TOKEN="<choose-a-long-random-admin-token>" `
    GOOGLE_CREDENTIALS_JSON="$GOOGLE_CREDENTIALS_JSON"
```

Azure stores App Service app settings encrypted at rest and injects them as environment variables when the container starts.

## 8. Restart And Verify Health

Restart the app:

```powershell
az webapp restart `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME
```

Get the public URL:

```powershell
$APP_URL = "https://$WEBAPP_NAME.azurewebsites.net"
$APP_URL
```

Check health:

```powershell
Invoke-WebRequest "$APP_URL/healthz"
```

Expected response:

```text
ok
```

## 9. Configure SeaTalk Callback URL

In the SeaTalk Open Platform app settings, set the event callback URL to:

```text
https://<WEBAPP_NAME>.azurewebsites.net/seatalk/callback
```

When SeaTalk verifies the callback URL, the server responds with the `seatalk_challenge` value from the verification event.

If verification fails:

- confirm `SEATALK_SIGNING_SECRET` is correct
- confirm the app is running at `/healthz`
- check Azure logs
- make sure the SeaTalk callback URL uses HTTPS

## 10. Test Manual Report Send

Run:

```powershell
$ADMIN_TOKEN = "<same-admin-token-you-set-in-azure>"

Invoke-WebRequest `
  -Method POST `
  -Uri "$APP_URL/admin/test-report" `
  -Headers @{ Authorization = "Bearer $ADMIN_TOKEN" }
```

Expected HTTP status:

```text
202 Accepted
```

This sends the interactive card to all group IDs in:

```text
BAU Backlogs Summary!A2:A
```

The generated report image is embedded inside the interactive message card.

## 11. View Logs

Stream logs:

```powershell
az webapp log tail `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME
```

If logs are not enabled:

```powershell
az webapp log config `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME `
  --docker-container-logging filesystem
```

Then restart and tail again:

```powershell
az webapp restart `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME

az webapp log tail `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME
```

## 12. Deploy Updates

When code changes, build and push a new tag:

```powershell
$IMAGE_TAG = "v2"

docker build -t "${IMAGE_NAME}:$IMAGE_TAG" .
docker tag "${IMAGE_NAME}:$IMAGE_TAG" "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG"
docker push "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG"
```

Point App Service to the new image:

```powershell
az webapp config container set `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME `
  --docker-custom-image-name "$ACR_LOGIN_SERVER/${IMAGE_NAME}:$IMAGE_TAG" `
  --docker-registry-server-url "https://$ACR_LOGIN_SERVER" `
  --docker-registry-server-user $ACR_USERNAME `
  --docker-registry-server-password $ACR_PASSWORD

az webapp restart `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME
```

## 13. Common Troubleshooting

### Health Check Fails

Check that these settings exist:

```text
WEBSITES_PORT=8080
PORT=8080
```

Then inspect logs:

```powershell
az webapp log tail --resource-group $RESOURCE_GROUP --name $WEBAPP_NAME
```

### Container Does Not Start

Common causes:

- missing SeaTalk or Google environment variables
- invalid `GOOGLE_CREDENTIALS_JSON`
- image pull failure from ACR
- App Service is pointing to the wrong image tag

Check container configuration:

```powershell
az webapp config container show `
  --resource-group $RESOURCE_GROUP `
  --name $WEBAPP_NAME
```

### SeaTalk Callback Verification Fails

Verify:

- callback URL is exactly `https://<WEBAPP_NAME>.azurewebsites.net/seatalk/callback`
- `SEATALK_SIGNING_SECRET` matches the SeaTalk app
- the server logs do not show `invalid signature`
- `/healthz` returns `ok`

### Google Sheets Reads Fail

Verify:

- the service account email has access to both spreadsheets
- `GOOGLE_CREDENTIALS_JSON` contains the full JSON
- `SPREADSHEET_ID` and `CARD_SPREADSHEET_ID` are correct
- sheet names and ranges match exactly

### Report Rendering Fails

The Docker image installs:

```text
poppler-utils
imagemagick
```

If rendering fails, inspect logs for `pdftoppm`, `magick`, or `convert` errors.

Large images can exceed SeaTalk limits. Lower these if needed:

```text
PNG_DPI=200
PNG_MAX_WIDTH=1600
```

### Watcher Does Not Send

The first watched-cell read is only a baseline and does not send. A send happens only after `WATCH_CELL` changes after startup.

For immediate testing, use:

```text
POST /admin/test-report
```

## 14. Clean Up Azure Resources

To delete everything created by this guide:

```powershell
az group delete `
  --name $RESOURCE_GROUP `
  --yes
```

## Reference

- Azure App Service app settings: https://learn.microsoft.com/azure/app-service/configure-common
- Azure App Service custom container tutorial: https://learn.microsoft.com/azure/app-service/tutorial-custom-container
- Azure custom container configuration: https://learn.microsoft.com/azure/app-service/containers/configure-custom-container
