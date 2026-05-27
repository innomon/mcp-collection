# SPEC.md: YouTube MCP Server Specification

## Overview
`mcp-youtube` is a Model Context Protocol (MCP) server written in Go 1.25+ that wraps the Google YouTube Data API v3 and YouTube Analytics API v2. It provides a standardized interface for AI models to query YouTube channel and video metadata, comments, and daily/aggregate analytics reports over both standard operations and mock dry-run simulations.

---

## 1. System Architecture

The server consists of the following modular layers:
- **CLI & Configuration (`config.go`)**: Custom CLI flag parsing (`-simulate`, `-data`, `-port`) using standard libraries combined with environmental config resolution.
- **Authentication Core (`auth.go`)**: Handles standard Google OAuth 2.0 flow:
  - Validates cached credentials at `~/.config/mcp-youtube/token.json`.
  - Spins up a temporary local HTTP server to receive redirect callbacks.
  - Exposes config-based port override (`-port` or `YOUTUBE_OAUTH_PORT`) with a safe default of `6050`.
  - Seamlessly handles background token refreshes using standard Go token wrappers.
- **Service Client (`client.go`)**: Standardizes interfaces, initializing official Google Client libraries (`youtube.v3` & `youtubeanalytics.v2`) under live mode, or routing mock calls in simulation mode.
- **MCP Tool Definitions (`tools.go`)**: Implements structural inputs and output serialization using reflection-based generic tool handlers (`mcp.AddTool`).

---

## 2. Configuration & Authentication Flow

### Environment Variables
- `YOUTUBE_OAUTH_CLIENT_ID`: Google OAuth 2.0 Client ID (Required in live mode).
- `YOUTUBE_OAUTH_CLIENT_SECRET`: Google OAuth 2.0 Client Secret (Required in live mode).
- `YOUTUBE_OAUTH_PORT`: The local HTTP callback port. Default fallback is `6050`.
- `YOUTUBE_TOKEN_CACHE_PATH`: Path to write the cached token JSON. Defaults to `~/.config/mcp-youtube/token.json`.

### CLI Flags
- `-simulate`: Toggles dry-run sandbox simulation using local `synthetic_data.json`.
- `-data <path>`: Specifies custom path for mock database (defaults to `synthetic_data.json`).
- `-port <int>`: Overrides environmental and default callback port configuration (e.g. `-port 9090`).

### Authentication Callback Mechanics
1. **Cache Lookups**: On startup, checks cached token at `YOUTUBE_TOKEN_CACHE_PATH`. If valid, it returns the HTTP client immediately.
2. **Server Spawning**: If invalid or missing, launches a lightweight HTTP callback server on port `OAuthPort` (`http://localhost:6050`).
3. **Link Presentation**: Outputs the secure Google Authorization link to `os.Stderr` (preventing protocol breakages on `stdout`).
4. **Consent & Exchange**: The user clicks the link, authenticates with Google, and gets redirected to `http://localhost:<port>/oauth2/callback?code=...&state=...`.
5. **Caching & Shutdown**: Exchanges `code` for an OAuth token, writes it back to `token.json`, and shuts down the callback server gracefully.

---

## 3. Tool Specifications

All registered tools are read-only.

### YouTube Data Tools

#### 1. `search_channels`
- **Description**: Search for channels matching a specific query.
- **Parameters**:
  - `query` (string, required): The search keyword or phrase.
- **Output**: Array of channels including `id`, `title`, `description`, `publishedAt`.

#### 2. `get_channel_details`
- **Description**: Retrieve detailed metadata and subscriber statistics.
- **Parameters**:
  - `channel_id` (string, required): Channel ID or handle (e.g. `@techgurus`).
- **Output**: `id`, `title`, `description`, `customUrl`, `publishedAt`, `subscriberCount`, `videoCount`, `viewCount`, `uploadsPlaylistId`.

#### 3. `search_videos`
- **Description**: Search for videos matching keywords, with optional sorting.
- **Parameters**:
  - `query` (string, required): The search query.
  - `order` (string, optional): Sort order (`date`, `rating`, `relevance`, `title`, `videoCount`, `viewCount`).
  - `max_results` (integer, optional): Maximum results (1-50, default 10).
- **Output**: Array of video snippets.

#### 4. `get_video_details`
- **Description**: Fetch statistics and full metadata for specific video IDs.
- **Parameters**:
  - `video_ids` (array of strings, required): List of video IDs.
- **Output**: Array containing views, likes, comments count, and durations.

#### 5. `list_channel_videos`
- **Description**: List uploaded videos for a specific channel ID.
- **Parameters**:
  - `channel_id` (string, required): Target channel ID.
  - `max_results` (integer, optional): Videos per page (default 10).
  - `page_token` (string, optional): Token to navigate pagination.
- **Output**: Video lists with a `next_page_token`.

#### 6. `get_video_comments`
- **Description**: Fetch top-level comments and comment threads for a video.
- **Parameters**:
  - `video_id` (string, required): Target video ID.
  - `max_results` (integer, optional): Comments to fetch (default 10).
- **Output**: Array containing `authorName`, `textDisplay`, `likeCount`, `publishedAt`.

---

### YouTube Analytics Tools

#### 7. `get_channel_analytics`
- **Description**: Retrieve channel performance aggregates (views, watch time, subscriber delta, avg duration) over a timeframe.
- **Parameters**:
  - `channel_id` (string, required): Target channel ID.
  - `start_date` (string, required): Start date (YYYY-MM-DD).
  - `end_date` (string, required): End date (YYYY-MM-DD).
- **Output**: Daily report table containing performance rows.

#### 8. `get_video_analytics`
- **Description**: Fetch daily aggregate metrics (views, watch time, likes, comments, shares) for a specific video ID.
- **Parameters**:
  - `channel_id` (string, optional): Owner channel ID (defaults to MINE).
  - `video_id` (string, required): Target video ID.
  - `start_date` (string, required): Start date (YYYY-MM-DD).
  - `end_date` (string, required): End date (YYYY-MM-DD).
- **Output**: Daily video performance report rows.

#### 9. `get_demographics_analytics`
- **Description**: Fetch demographics statistics (age group and gender shares).
- **Parameters**:
  - `channel_id` (string, required): Target channel ID.
  - `start_date` (string, required): Start date (YYYY-MM-DD).
  - `end_date` (string, required): End date (YYYY-MM-DD).
- **Output**: Demographic breakdown records (`ageGroup`, `gender`, `viewerPercentage`).

#### 10. `get_traffic_source_analytics`
- **Description**: Query acquisition reports showing where viewers found videos.
- **Parameters**:
  - `channel_id` (string, required): Target channel ID.
  - `start_date` (string, required): Start date (YYYY-MM-DD).
  - `end_date` (string, required): End date (YYYY-MM-DD).
- **Output**: Traffic source referral records (`sourceType`, `views`, `watchTimeMinutes`).
