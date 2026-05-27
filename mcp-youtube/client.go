package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/api/youtubeanalytics/v2"
)

type YouTubeClient struct {
	cfg          *Config
	dataSvc      *youtube.Service
	analyticsSvc *youtubeanalytics.Service
	mockData     *SyntheticData
}

type SyntheticData struct {
	Channels         map[string]map[string]any   `json:"channels"`
	Videos           map[string]map[string]any   `json:"videos"`
	ChannelAnalytics map[string][]map[string]any `json:"channel_analytics"`
	VideoAnalytics   map[string][]map[string]any `json:"video_analytics"`
	Demographics     map[string][]map[string]any `json:"demographics"`
	TrafficSources   map[string][]map[string]any `json:"traffic_sources"`
	VideoComments    map[string][]map[string]any `json:"video_comments"`
}

func NewYouTubeClient(ctx context.Context, cfg *Config) (*YouTubeClient, error) {
	client := &YouTubeClient{cfg: cfg}

	if cfg.Simulate {
		if err := client.loadSyntheticData(cfg.DataFile); err != nil {
			return nil, fmt.Errorf("loading synthetic data: %w", err)
		}
		log.Printf("YouTube Client: Initialized in SIMULATION mode.")
		return client, nil
	}

	// Live mode: authenticate and create clients
	httpClient, err := getOAuthClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("oauth initialization: %w", err)
	}

	dataSvc, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating YouTube Data service: %w", err)
	}

	analyticsSvc, err := youtubeanalytics.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating YouTube Analytics service: %w", err)
	}

	client.dataSvc = dataSvc
	client.analyticsSvc = analyticsSvc
	log.Printf("YouTube Client: Initialized in LIVE mode.")
	return client, nil
}

func (c *YouTubeClient) loadSyntheticData(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	var data SyntheticData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	c.mockData = &data
	return nil
}

// ============================================================================
// 1. YouTube Data API Methods
// ============================================================================

func (c *YouTubeClient) SearchChannels(ctx context.Context, query string) ([]map[string]any, error) {
	if c.cfg.Simulate {
		var results []map[string]any
		q := strings.ToLower(query)
		for _, channel := range c.mockData.Channels {
			title, _ := channel["title"].(string)
			desc, _ := channel["description"].(string)
			if strings.Contains(strings.ToLower(title), q) || strings.Contains(strings.ToLower(desc), q) {
				results = append(results, channel)
			}
		}
		return results, nil
	}

	call := c.dataSvc.Search.List([]string{"snippet"}).Q(query).Type("channel").MaxResults(10)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, item := range resp.Items {
		results = append(results, map[string]any{
			"id":          item.Id.ChannelId,
			"title":       item.Snippet.Title,
			"description": item.Snippet.Description,
			"publishedAt": item.Snippet.PublishedAt,
		})
	}
	return results, nil
}

func (c *YouTubeClient) GetChannelDetails(ctx context.Context, channelID string) (map[string]any, error) {
	if c.cfg.Simulate {
		if channel, ok := c.mockData.Channels[channelID]; ok {
			return channel, nil
		}
		// Try search by username
		for _, channel := range c.mockData.Channels {
			customUrl, _ := channel["customUrl"].(string)
			if strings.EqualFold(customUrl, channelID) || strings.EqualFold(customUrl, "@"+channelID) {
				return channel, nil
			}
		}
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}

	// Determine if input is a handle (starts with @)
	call := c.dataSvc.Channels.List([]string{"snippet", "statistics", "contentDetails"})
	if strings.HasPrefix(channelID, "@") {
		call = call.ForHandle(channelID)
	} else {
		call = call.Id(channelID)
	}

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}

	item := resp.Items[0]
	return map[string]any{
		"id":                item.Id,
		"title":             item.Snippet.Title,
		"description":       item.Snippet.Description,
		"customUrl":         item.Snippet.CustomUrl,
		"publishedAt":       item.Snippet.PublishedAt,
		"subscriberCount":   item.Statistics.SubscriberCount,
		"videoCount":        item.Statistics.VideoCount,
		"viewCount":         item.Statistics.ViewCount,
		"uploadsPlaylistId": item.ContentDetails.RelatedPlaylists.Uploads,
	}, nil
}

func (c *YouTubeClient) SearchVideos(ctx context.Context, query string, order string, maxResults int64) ([]map[string]any, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if c.cfg.Simulate {
		var results []map[string]any
		q := strings.ToLower(query)
		for _, video := range c.mockData.Videos {
			title, _ := video["title"].(string)
			desc, _ := video["description"].(string)
			if strings.Contains(strings.ToLower(title), q) || strings.Contains(strings.ToLower(desc), q) {
				results = append(results, video)
			}
		}
		if int64(len(results)) > maxResults {
			results = results[:maxResults]
		}
		return results, nil
	}

	call := c.dataSvc.Search.List([]string{"snippet"}).Q(query).Type("video").MaxResults(maxResults)
	if order != "" {
		call = call.Order(order)
	}
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, item := range resp.Items {
		results = append(results, map[string]any{
			"id":           item.Id.VideoId,
			"title":        item.Snippet.Title,
			"description":  item.Snippet.Description,
			"publishedAt":  item.Snippet.PublishedAt,
			"channelId":    item.Snippet.ChannelId,
			"channelTitle": item.Snippet.ChannelTitle,
		})
	}
	return results, nil
}

func (c *YouTubeClient) GetVideoDetails(ctx context.Context, videoIDs []string) ([]map[string]any, error) {
	if c.cfg.Simulate {
		var results []map[string]any
		for _, id := range videoIDs {
			if video, ok := c.mockData.Videos[id]; ok {
				results = append(results, video)
			}
		}
		return results, nil
	}

	call := c.dataSvc.Videos.List([]string{"snippet", "statistics", "contentDetails"}).Id(strings.Join(videoIDs, ","))
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, item := range resp.Items {
		results = append(results, map[string]any{
			"id":           item.Id,
			"title":        item.Snippet.Title,
			"description":  item.Snippet.Description,
			"publishedAt":  item.Snippet.PublishedAt,
			"channelId":    item.Snippet.ChannelId,
			"channelTitle": item.Snippet.ChannelTitle,
			"viewCount":    item.Statistics.ViewCount,
			"likeCount":    item.Statistics.LikeCount,
			"commentCount": item.Statistics.CommentCount,
			"duration":     item.ContentDetails.Duration,
		})
	}
	return results, nil
}

func (c *YouTubeClient) ListChannelVideos(ctx context.Context, channelID string, maxResults int64, pageToken string) ([]map[string]any, string, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if c.cfg.Simulate {
		var results []map[string]any
		for _, video := range c.mockData.Videos {
			chID, _ := video["channelId"].(string)
			if chID == channelID {
				results = append(results, video)
			}
		}
		if int64(len(results)) > maxResults {
			results = results[:maxResults]
		}
		return results, "", nil
	}

	// 1. Get the uploads playlist ID first
	chDetails, err := c.GetChannelDetails(ctx, channelID)
	if err != nil {
		return nil, "", fmt.Errorf("getting channel details for uploads playlist: %w", err)
	}

	uploadsPlaylistID, ok := chDetails["uploadsPlaylistId"].(string)
	if !ok || uploadsPlaylistID == "" {
		return nil, "", fmt.Errorf("uploads playlist ID not found for channel: %s", channelID)
	}

	// 2. Query playlist items
	call := c.dataSvc.PlaylistItems.List([]string{"snippet", "contentDetails"}).
		PlaylistId(uploadsPlaylistID).
		MaxResults(maxResults).
		PageToken(pageToken)

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, "", err
	}

	var results []map[string]any
	for _, item := range resp.Items {
		results = append(results, map[string]any{
			"id":           item.ContentDetails.VideoId,
			"title":        item.Snippet.Title,
			"description":  item.Snippet.Description,
			"publishedAt":  item.Snippet.PublishedAt,
			"channelId":    item.Snippet.ChannelId,
			"channelTitle": item.Snippet.ChannelTitle,
		})
	}
	return results, resp.NextPageToken, nil
}

func (c *YouTubeClient) GetVideoComments(ctx context.Context, videoID string, maxResults int64) ([]map[string]any, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if c.cfg.Simulate {
		if comments, ok := c.mockData.VideoComments[videoID]; ok {
			if int64(len(comments)) > maxResults {
				return comments[:maxResults], nil
			}
			return comments, nil
		}
		return []map[string]any{}, nil
	}

	call := c.dataSvc.CommentThreads.List([]string{"snippet"}).VideoId(videoID).MaxResults(maxResults)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		// YouTube returns 403 or 400 if comments are disabled
		return nil, err
	}

	var results []map[string]any
	for _, item := range resp.Items {
		comment := item.Snippet.TopLevelComment
		results = append(results, map[string]any{
			"id":               comment.Id,
			"authorName":       comment.Snippet.AuthorDisplayName,
			"authorChannelUrl": comment.Snippet.AuthorChannelUrl,
			"textDisplay":      comment.Snippet.TextDisplay,
			"likeCount":        comment.Snippet.LikeCount,
			"publishedAt":      comment.Snippet.PublishedAt,
		})
	}
	return results, nil
}

// ============================================================================
// 2. YouTube Analytics API Methods
// ============================================================================

func (c *YouTubeClient) GetChannelAnalytics(ctx context.Context, channelID string, startDate string, endDate string) ([]map[string]any, error) {
	if c.cfg.Simulate {
		if analytics, ok := c.mockData.ChannelAnalytics[channelID]; ok {
			return analytics, nil
		}
		// Fallback to first available channel analytics
		for _, v := range c.mockData.ChannelAnalytics {
			return v, nil
		}
		return []map[string]any{}, nil
	}

	call := c.analyticsSvc.Reports.Query().
		Ids("channel==" + channelID).
		StartDate(startDate).
		EndDate(endDate).
		Metrics("views,watchTimeMinutes,subscribersGained,subscribersLost,averageViewDuration").
		Dimensions("day")

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return formatReportResponse(resp)
}

func (c *YouTubeClient) GetVideoAnalytics(ctx context.Context, channelID string, videoID string, startDate string, endDate string) ([]map[string]any, error) {
	if c.cfg.Simulate {
		if analytics, ok := c.mockData.VideoAnalytics[videoID]; ok {
			return analytics, nil
		}
		// Fallback
		for _, v := range c.mockData.VideoAnalytics {
			return v, nil
		}
		return []map[string]any{}, nil
	}

	// In YouTube Analytics, queries are framed against a channel (MINE or specific ID) and filtered by video ID
	targetChannel := channelID
	if targetChannel == "" {
		targetChannel = "MINE"
	}

	call := c.analyticsSvc.Reports.Query().
		Ids("channel==" + targetChannel).
		StartDate(startDate).
		EndDate(endDate).
		Metrics("views,watchTimeMinutes,likes,comments,shares").
		Dimensions("day").
		Filters("video==" + videoID)

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return formatReportResponse(resp)
}

func (c *YouTubeClient) GetDemographicsAnalytics(ctx context.Context, channelID string, startDate string, endDate string) ([]map[string]any, error) {
	if c.cfg.Simulate {
		if demo, ok := c.mockData.Demographics[channelID]; ok {
			return demo, nil
		}
		// Fallback
		for _, v := range c.mockData.Demographics {
			return v, nil
		}
		return []map[string]any{}, nil
	}

	call := c.analyticsSvc.Reports.Query().
		Ids("channel==" + channelID).
		StartDate(startDate).
		EndDate(endDate).
		Metrics("viewerPercentage").
		Dimensions("ageGroup,gender")

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return formatReportResponse(resp)
}

func (c *YouTubeClient) GetTrafficSourceAnalytics(ctx context.Context, channelID string, startDate string, endDate string) ([]map[string]any, error) {
	if c.cfg.Simulate {
		if traffic, ok := c.mockData.TrafficSources[channelID]; ok {
			return traffic, nil
		}
		// Fallback
		for _, v := range c.mockData.TrafficSources {
			return v, nil
		}
		return []map[string]any{}, nil
	}

	call := c.analyticsSvc.Reports.Query().
		Ids("channel==" + channelID).
		StartDate(startDate).
		EndDate(endDate).
		Metrics("views,watchTimeMinutes").
		Dimensions("insightTrafficSourceType")

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return formatReportResponse(resp)
}

// Helper to format Google Analytics report rows into JSON maps
func formatReportResponse(resp *youtubeanalytics.QueryResponse) ([]map[string]any, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return []map[string]any{}, nil
	}

	var results []map[string]any
	headers := make([]string, len(resp.ColumnHeaders))
	for i, h := range resp.ColumnHeaders {
		headers[i] = h.Name
	}

	for _, row := range resp.Rows {
		record := make(map[string]any)
		for idx, val := range row {
			if idx < len(headers) {
				record[headers[idx]] = val
			}
		}
		results = append(results, record)
	}

	return results, nil
}
