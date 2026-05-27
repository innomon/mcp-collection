# YouTube & YouTube Analytics MCP Server

A Model Context Protocol (MCP) server written in Go 1.25+ that exposes the Google YouTube Data API v3 and YouTube Analytics API v2. It allows AI models to search for channels and videos, retrieve video details, fetch comment threads, and pull extensive analytics reports (performance, demographics, and traffic sources).

It supports both a fully live OAuth 2.0 connection mode and a mock-driven, local dry-run simulation mode.

---

## Features

- **Google OAuth 2.0 Integration**: Automatically prompts, exchanges, caches, and refreshes OAuth tokens locally for seamless, secure interactions.
- **Strict Read-Only Operations**: Restricts access strictly to data queries and statistics reading to prevent accidental writes.
- **Simulation Mode**: Run dry-run commands immediately with mock statistics (`synthetic_data.json`) without needing active Google developer credentials.
- **Port Fallback Control**: Customize the OAuth callback server port with the `-port` flag or `YOUTUBE_OAUTH_PORT` environment variable (defaults to `6050`).

---

## 1. Setup & Installation

### Build Server
Navigate to the directory and compile the server executable:
```bash
cd mcp-youtube
go build -o mcp-youtube .
```

---

## 2. Configuration for Live Mode

To run in live mode, you must obtain OAuth 2.0 credentials from the Google Cloud Console.

### Google Cloud Project Setup
1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new Project (or select an existing one).
3. Enable APIs:
   - Search for and enable **YouTube Data API v3**.
   - Search for and enable **YouTube Analytics API**.
4. Configure the OAuth Consent Screen:
   - Set User Type to **External** (or Internal if under a Google Workspace).
   - Fill in app metadata.
   - Add Scopes:
     - `.../auth/youtube.readonly`
     - `.../auth/yt-analytics.readonly`
     - `.../auth/yt-analytics-monetization.readonly`
   - Add your own email as a Test User (required while the app is in "Testing" mode).
5. Generate Credentials:
   - Click **Create Credentials** -> **OAuth Client ID**.
   - Select application type **Web Application** (required to support HTTP loopback callbacks).
   - In **Authorized Redirect URIs**, add exactly:
     `http://localhost:6050/oauth2/callback`
     *(Note: If you customize the port to e.g. 9000, add `http://localhost:9000/oauth2/callback` instead)*
   - Save and copy your **Client ID** and **Client Secret**.

---

## 3. Usage

### Simulation / Mock Mode (No Setup Required)
Run the server instantly in simulation mode using local synthetic records:
```bash
./mcp-youtube -simulate -data synthetic_data.json
```

### Live Mode
Set the required environment variables and run:
```bash
export YOUTUBE_OAUTH_CLIENT_ID="your_client_id.apps.googleusercontent.com"
export YOUTUBE_OAUTH_CLIENT_SECRET="your_client_secret"
./mcp-youtube
```
On the first run (or when the cache at `~/.config/mcp-youtube/token.json` is missing/expired), the server will spin up a local HTTP callback listener at `http://localhost:6050` and write an authorization URL to `os.Stderr`. Open that URL in your browser, complete the Google consent screen, and the server will cache your token for all future sessions.

---

## 4. Integration with Claude Desktop (or other MCP Clients)

To use this server with Claude Desktop, add it to your configuration file (located at `~/.config/Claude/claude_desktop_config.json` on macOS/Linux):

### Live Configuration
```json
{
  "mcpServers": {
    "youtube": {
      "command": "/absolute/path/to/mcp-youtube/mcp-youtube",
      "env": {
        "YOUTUBE_OAUTH_CLIENT_ID": "your_client_id.apps.googleusercontent.com",
        "YOUTUBE_OAUTH_CLIENT_SECRET": "your_client_secret",
        "YOUTUBE_OAUTH_PORT": "6050"
      }
    }
  }
}
```

### Simulation/Sandbox Configuration
```json
{
  "mcpServers": {
    "youtube-sandbox": {
      "command": "/absolute/path/to/mcp-youtube/mcp-youtube",
      "args": ["-simulate", "-data", "/absolute/path/to/mcp-youtube/synthetic_data.json"]
    }
  }
}
```

---

## 5. Available Tools

### YouTube Data API
- `search_channels`: Search for channels by query.
- `get_channel_details`: Fetch channel snippet metadata, custom URLs, and statistics.
- `search_videos`: Search for videos with sorting/filtering parameters.
- `get_video_details`: Fetch metadata (durations, stats) for specific videos.
- `list_channel_videos`: Retrieve all uploaded videos from a channel.
- `get_video_comments`: Fetch comments and thread responses for a video.

### YouTube Analytics API
- `get_channel_analytics`: Retrieve aggregate channel metrics (views, watch time, subscribers, etc.).
- `get_video_analytics`: Fetch views, likes, comments, and shares for target videos.
- `get_demographics_analytics`: Fetch demographics (age group and gender shares).
- `get_traffic_source_analytics`: Fetch traffic referral and source acquisition reports.
