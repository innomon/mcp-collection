package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolHandler struct {
	client *YouTubeClient
}

func RegisterTools(server *mcp.Server, client *YouTubeClient) {
	h := &ToolHandler{client: client}

	// ============================================================================
	// 1. YouTube Data API Tools
	// ============================================================================

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_channels",
		Description: "Search for channels matching a specific query.",
	}, h.SearchChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_channel_details",
		Description: "Retrieve detailed metadata for a channel by channel ID or username/handle (e.g., '@techgurus').",
	}, h.GetChannelDetails)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_videos",
		Description: "Search for videos by keyword query, with optional sorting criteria.",
	}, h.SearchVideos)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_video_details",
		Description: "Retrieve complete metadata, statistics, and duration for specific video IDs.",
	}, h.GetVideoDetails)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_channel_videos",
		Description: "List all uploaded videos for a channel by paginating its default uploads playlist.",
	}, h.ListChannelVideos)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_video_comments",
		Description: "Retrieve top-level comments and threads for a specific video.",
	}, h.GetVideoComments)

	// ============================================================================
	// 2. YouTube Analytics API Tools
	// ============================================================================

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_channel_analytics",
		Description: "Fetch aggregate channel performance metrics (views, watch time, subscriber delta, avg duration) over a date range.",
	}, h.GetChannelAnalytics)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_video_analytics",
		Description: "Fetch daily metrics (views, watch time, likes, comments, shares) for a specific video ID over a date range.",
	}, h.GetVideoAnalytics)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_demographics_analytics",
		Description: "Query viewer age group, gender, and geographic breakdowns over a date range.",
	}, h.GetDemographicsAnalytics)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_traffic_source_analytics",
		Description: "Query traffic acquisition reports showing where viewers found the channel's videos.",
	}, h.GetTrafficSourceAnalytics)
}

// ============================================================================
// Data API Structures & Handlers
// ============================================================================

type SearchChannelsInput struct {
	Query string `json:"query" jsonschema:"Search query keyword or phrase"`
}

type SearchChannelsOutput struct {
	Channels []map[string]any `json:"channels" jsonschema:"List of matching channels"`
}

func (h *ToolHandler) SearchChannels(ctx context.Context, req *mcp.CallToolRequest, input SearchChannelsInput) (*mcp.CallToolResult, SearchChannelsOutput, error) {
	log.Printf("MCP Tool Call: SearchChannels query=%q", input.Query)
	if input.Query == "" {
		return toolError("invalid_argument", "query is required"), SearchChannelsOutput{}, nil
	}

	results, err := h.client.SearchChannels(ctx, input.Query)
	if err != nil {
		return toolError("api_error", err.Error()), SearchChannelsOutput{}, nil
	}
	return nil, SearchChannelsOutput{Channels: results}, nil
}

type GetChannelDetailsInput struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID (e.g. UC12345) or handle (e.g. @techgurus)"`
}

type GetChannelDetailsOutput struct {
	Channel map[string]any `json:"channel" jsonschema:"Channel details and statistics"`
}

func (h *ToolHandler) GetChannelDetails(ctx context.Context, req *mcp.CallToolRequest, input GetChannelDetailsInput) (*mcp.CallToolResult, GetChannelDetailsOutput, error) {
	log.Printf("MCP Tool Call: GetChannelDetails channel_id=%q", input.ChannelID)
	if input.ChannelID == "" {
		return toolError("invalid_argument", "channel_id is required"), GetChannelDetailsOutput{}, nil
	}

	result, err := h.client.GetChannelDetails(ctx, input.ChannelID)
	if err != nil {
		return toolError("api_error", err.Error()), GetChannelDetailsOutput{}, nil
	}
	return nil, GetChannelDetailsOutput{Channel: result}, nil
}

type SearchVideosInput struct {
	Query      string `json:"query" jsonschema:"Search query keyword or phrase"`
	Order      string `json:"order,omitempty" jsonschema:"Sort order: date, rating, relevance (default), title, videoCount, viewCount"`
	MaxResults int64  `json:"max_results,omitempty" jsonschema:"Max number of records to return (default 10, max 50)"`
}

type SearchVideosOutput struct {
	Videos []map[string]any `json:"videos" jsonschema:"List of matching videos"`
}

func (h *ToolHandler) SearchVideos(ctx context.Context, req *mcp.CallToolRequest, input SearchVideosInput) (*mcp.CallToolResult, SearchVideosOutput, error) {
	log.Printf("MCP Tool Call: SearchVideos query=%q order=%q", input.Query, input.Order)
	if input.Query == "" {
		return toolError("invalid_argument", "query is required"), SearchVideosOutput{}, nil
	}

	results, err := h.client.SearchVideos(ctx, input.Query, input.Order, input.MaxResults)
	if err != nil {
		return toolError("api_error", err.Error()), SearchVideosOutput{}, nil
	}
	return nil, SearchVideosOutput{Videos: results}, nil
}

type GetVideoDetailsInput struct {
	VideoIDs []string `json:"video_ids" jsonschema:"List of video IDs to retrieve details for"`
}

type GetVideoDetailsOutput struct {
	Videos []map[string]any `json:"videos" jsonschema:"Details and statistics of requested videos"`
}

func (h *ToolHandler) GetVideoDetails(ctx context.Context, req *mcp.CallToolRequest, input GetVideoDetailsInput) (*mcp.CallToolResult, GetVideoDetailsOutput, error) {
	log.Printf("MCP Tool Call: GetVideoDetails ids=%v", input.VideoIDs)
	if len(input.VideoIDs) == 0 {
		return toolError("invalid_argument", "video_ids is required"), GetVideoDetailsOutput{}, nil
	}

	results, err := h.client.GetVideoDetails(ctx, input.VideoIDs)
	if err != nil {
		return toolError("api_error", err.Error()), GetVideoDetailsOutput{}, nil
	}
	return nil, GetVideoDetailsOutput{Videos: results}, nil
}

type ListChannelVideosInput struct {
	ChannelID  string `json:"channel_id" jsonschema:"Channel ID to retrieve uploaded videos for"`
	MaxResults int64  `json:"max_results,omitempty" jsonschema:"Max number of videos to fetch (default 10)"`
	PageToken  string `json:"page_token,omitempty" jsonschema:"Token for retrieving the next page"`
}

type ListChannelVideosOutput struct {
	Videos        []map[string]any `json:"videos" jsonschema:"List of videos uploaded by channel"`
	NextPageToken string           `json:"next_page_token,omitempty" jsonschema:"Token for next page results"`
}

func (h *ToolHandler) ListChannelVideos(ctx context.Context, req *mcp.CallToolRequest, input ListChannelVideosInput) (*mcp.CallToolResult, ListChannelVideosOutput, error) {
	log.Printf("MCP Tool Call: ListChannelVideos channel_id=%q limit=%d", input.ChannelID, input.MaxResults)
	if input.ChannelID == "" {
		return toolError("invalid_argument", "channel_id is required"), ListChannelVideosOutput{}, nil
	}

	results, nextToken, err := h.client.ListChannelVideos(ctx, input.ChannelID, input.MaxResults, input.PageToken)
	if err != nil {
		return toolError("api_error", err.Error()), ListChannelVideosOutput{}, nil
	}
	return nil, ListChannelVideosOutput{Videos: results, NextPageToken: nextToken}, nil
}

type GetVideoCommentsInput struct {
	VideoID    string `json:"video_id" jsonschema:"Video ID to fetch comments for"`
	MaxResults int64  `json:"max_results,omitempty" jsonschema:"Max number of comments to fetch (default 10)"`
}

type GetVideoCommentsOutput struct {
	Comments []map[string]any `json:"comments" jsonschema:"Top-level comments list"`
}

func (h *ToolHandler) GetVideoComments(ctx context.Context, req *mcp.CallToolRequest, input GetVideoCommentsInput) (*mcp.CallToolResult, GetVideoCommentsOutput, error) {
	log.Printf("MCP Tool Call: GetVideoComments video_id=%q limit=%d", input.VideoID, input.MaxResults)
	if input.VideoID == "" {
		return toolError("invalid_argument", "video_id is required"), GetVideoCommentsOutput{}, nil
	}

	results, err := h.client.GetVideoComments(ctx, input.VideoID, input.MaxResults)
	if err != nil {
		return toolError("api_error", err.Error()), GetVideoCommentsOutput{}, nil
	}
	return nil, GetVideoCommentsOutput{Comments: results}, nil
}

// ============================================================================
// Analytics API Structures & Handlers
// ============================================================================

type GetChannelAnalyticsInput struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID to fetch reports for"`
	StartDate string `json:"start_date" jsonschema:"Start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"End date in YYYY-MM-DD format"`
}

type GetChannelAnalyticsOutput struct {
	Report []map[string]any `json:"report" jsonschema:"Daily aggregate channel performance report"`
}

func (h *ToolHandler) GetChannelAnalytics(ctx context.Context, req *mcp.CallToolRequest, input GetChannelAnalyticsInput) (*mcp.CallToolResult, GetChannelAnalyticsOutput, error) {
	log.Printf("MCP Tool Call: GetChannelAnalytics channel_id=%q range=%s to %s", input.ChannelID, input.StartDate, input.EndDate)
	if input.ChannelID == "" || input.StartDate == "" || input.EndDate == "" {
		return toolError("invalid_argument", "channel_id, start_date, and end_date are required"), GetChannelAnalyticsOutput{}, nil
	}

	results, err := h.client.GetChannelAnalytics(ctx, input.ChannelID, input.StartDate, input.EndDate)
	if err != nil {
		return toolError("api_error", err.Error()), GetChannelAnalyticsOutput{}, nil
	}
	return nil, GetChannelAnalyticsOutput{Report: results}, nil
}

type GetVideoAnalyticsInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"Channel ID that owns the video (defaults to MINE)"`
	VideoID   string `json:"video_id" jsonschema:"Video ID to fetch reports for"`
	StartDate string `json:"start_date" jsonschema:"Start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"End date in YYYY-MM-DD format"`
}

type GetVideoAnalyticsOutput struct {
	Report []map[string]any `json:"report" jsonschema:"Daily metrics report for the target video"`
}

func (h *ToolHandler) GetVideoAnalytics(ctx context.Context, req *mcp.CallToolRequest, input GetVideoAnalyticsInput) (*mcp.CallToolResult, GetVideoAnalyticsOutput, error) {
	log.Printf("MCP Tool Call: GetVideoAnalytics video_id=%q range=%s to %s", input.VideoID, input.StartDate, input.EndDate)
	if input.VideoID == "" || input.StartDate == "" || input.EndDate == "" {
		return toolError("invalid_argument", "video_id, start_date, and end_date are required"), GetVideoAnalyticsOutput{}, nil
	}

	results, err := h.client.GetVideoAnalytics(ctx, input.ChannelID, input.VideoID, input.StartDate, input.EndDate)
	if err != nil {
		return toolError("api_error", err.Error()), GetVideoAnalyticsOutput{}, nil
	}
	return nil, GetVideoAnalyticsOutput{Report: results}, nil
}

type GetDemographicsAnalyticsInput struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID to fetch demographics for"`
	StartDate string `json:"start_date" jsonschema:"Start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"End date in YYYY-MM-DD format"`
}

type GetDemographicsAnalyticsOutput struct {
	Report []map[string]any `json:"report" jsonschema:"Age group and gender breakdowns"`
}

func (h *ToolHandler) GetDemographicsAnalytics(ctx context.Context, req *mcp.CallToolRequest, input GetDemographicsAnalyticsInput) (*mcp.CallToolResult, GetDemographicsAnalyticsOutput, error) {
	log.Printf("MCP Tool Call: GetDemographicsAnalytics channel_id=%q range=%s to %s", input.ChannelID, input.StartDate, input.EndDate)
	if input.ChannelID == "" || input.StartDate == "" || input.EndDate == "" {
		return toolError("invalid_argument", "channel_id, start_date, and end_date are required"), GetDemographicsAnalyticsOutput{}, nil
	}

	results, err := h.client.GetDemographicsAnalytics(ctx, input.ChannelID, input.StartDate, input.EndDate)
	if err != nil {
		return toolError("api_error", err.Error()), GetDemographicsAnalyticsOutput{}, nil
	}
	return nil, GetDemographicsAnalyticsOutput{Report: results}, nil
}

type GetTrafficSourceAnalyticsInput struct {
	ChannelID string `json:"channel_id" jsonschema:"Channel ID to fetch traffic sources for"`
	StartDate string `json:"start_date" jsonschema:"Start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"End date in YYYY-MM-DD format"`
}

type GetTrafficSourceAnalyticsOutput struct {
	Report []map[string]any `json:"report" jsonschema:"Traffic referral and source report"`
}

func (h *ToolHandler) GetTrafficSourceAnalytics(ctx context.Context, req *mcp.CallToolRequest, input GetTrafficSourceAnalyticsInput) (*mcp.CallToolResult, GetTrafficSourceAnalyticsOutput, error) {
	log.Printf("MCP Tool Call: GetTrafficSourceAnalytics channel_id=%q range=%s to %s", input.ChannelID, input.StartDate, input.EndDate)
	if input.ChannelID == "" || input.StartDate == "" || input.EndDate == "" {
		return toolError("invalid_argument", "channel_id, start_date, and end_date are required"), GetTrafficSourceAnalyticsOutput{}, nil
	}

	results, err := h.client.GetTrafficSourceAnalytics(ctx, input.ChannelID, input.StartDate, input.EndDate)
	if err != nil {
		return toolError("api_error", err.Error()), GetTrafficSourceAnalyticsOutput{}, nil
	}
	return nil, GetTrafficSourceAnalyticsOutput{Report: results}, nil
}

// ============================================================================
// Tool Helpers
// ============================================================================

func toolError(code string, msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error [%s]: %s", code, msg)},
		},
	}
}
