package main

import (
	"context"
	"testing"
)

func TestSimulationClient(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		Simulate:       true,
		DataFile:       "synthetic_data.json",
		OAuthPort:      6050,
		TokenCachePath: "token.json",
	}

	client, err := NewYouTubeClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 1. Test SearchChannels
	channels, err := client.SearchChannels(ctx, "Tech")
	if err != nil {
		t.Fatalf("SearchChannels failed: %v", err)
	}
	if len(channels) == 0 {
		t.Errorf("expected at least one channel, got 0")
	}
	if channels[0]["title"] != "TechGurus" {
		t.Errorf("expected channel title TechGurus, got %v", channels[0]["title"])
	}

	// 2. Test GetChannelDetails
	chDetail, err := client.GetChannelDetails(ctx, "UC1234567890")
	if err != nil {
		t.Fatalf("GetChannelDetails failed: %v", err)
	}
	if chDetail["title"] != "TechGurus" {
		t.Errorf("expected TechGurus, got %v", chDetail["title"])
	}

	// 3. Test SearchVideos
	videos, err := client.SearchVideos(ctx, "Go 1.25", "", 5)
	if err != nil {
		t.Fatalf("SearchVideos failed: %v", err)
	}
	if len(videos) == 0 {
		t.Errorf("expected at least one video, got 0")
	}

	// 4. Test GetVideoDetails
	vDetails, err := client.GetVideoDetails(ctx, []string{"v_go125"})
	if err != nil {
		t.Fatalf("GetVideoDetails failed: %v", err)
	}
	if len(vDetails) != 1 {
		t.Errorf("expected 1 video detail, got %d", len(vDetails))
	}
	if vDetails[0]["title"] != "Go 1.25 Features Deep Dive: Typed Tool Registry & New Routing" {
		t.Errorf("unexpected video title: %v", vDetails[0]["title"])
	}

	// 5. Test GetChannelAnalytics
	analytics, err := client.GetChannelAnalytics(ctx, "UC1234567890", "2026-05-20", "2026-05-24")
	if err != nil {
		t.Fatalf("GetChannelAnalytics failed: %v", err)
	}
	if len(analytics) == 0 {
		t.Errorf("expected channel analytics rows, got 0")
	}

	// 6. Test GetDemographicsAnalytics
	demo, err := client.GetDemographicsAnalytics(ctx, "UC1234567890", "2026-05-20", "2026-05-24")
	if err != nil {
		t.Fatalf("GetDemographicsAnalytics failed: %v", err)
	}
	if len(demo) == 0 {
		t.Errorf("expected demographics rows, got 0")
	}
}
