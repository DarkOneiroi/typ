package search

import (
	"context"
	"testing"
)

func TestSearch(t *testing.T) {
	s, err := NewYtDlpSearcher()
	if err != nil {
		t.Skip("yt-dlp not found, skipping search test")
	}
	_, _ = s.Search(context.Background(), "test", ResultTypeVideo, 1)
}

func TestMapToResult(t *testing.T) {
	s := &YtDlpSearcher{}

	tests := []struct {
		name     string
		entry    ytDlpEntry
		resType  ResultType
		expected Result
	}{
		{
			name: "Basic Video Mapping",
			entry: ytDlpEntry{
				ID:        "v1",
				Title:     "Test Video",
				Uploader:  "Test Channel",
				Duration:  125,
				ViewCount: 1500,
			},
			resType: ResultTypeVideo,
			expected: Result{
				ID:          "v1",
				Type:        ResultTypeVideo,
				Title:       "Test Video",
				ChannelName: "Test Channel",
				Duration:    "2:05",
				Views:       "1.5K views",
			},
		},
		{
			name: "Channel Mapping with Subs",
			entry: ytDlpEntry{
				ID:              "c1",
				Title:           "Test Channel",
				SubscriberCount: 1200000,
			},
			resType: ResultTypeChannel,
			expected: Result{
				ID:          "c1",
				Type:        ResultTypeChannel,
				Title:       "Test Channel",
				ChannelName: "Unknown Channel",
				VideoCount:  "1.2M subs",
			},
		},
		{
			name: "Thumbnail Selection",
			entry: ytDlpEntry{
				ID:    "v2",
				Title: "Thumb Test",
				Thumbnails: []ytDlpThumbnail{
					{URL: "low", Width: 100, Height: 100},
					{URL: "high", Width: 1920, Height: 1080},
				},
			},
			resType: ResultTypeVideo,
			expected: Result{
				ID:        "v2",
				Type:      ResultTypeVideo,
				Title:     "Thumb Test",
				Thumbnail: "high",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.mapToResult(tt.entry, tt.resType)
			if got.ID != tt.expected.ID {
				t.Errorf("expected ID %s, got %s", tt.expected.ID, got.ID)
			}
			if got.Title != tt.expected.Title {
				t.Errorf("expected Title %s, got %s", tt.expected.Title, got.Title)
			}
			if tt.expected.Duration != "" && got.Duration != tt.expected.Duration {
				t.Errorf("expected Duration %s, got %s", tt.expected.Duration, got.Duration)
			}
			if tt.expected.Views != "" && got.Views != tt.expected.Views {
				t.Errorf("expected Views %s, got %s", tt.expected.Views, got.Views)
			}
			if tt.expected.VideoCount != "" && got.VideoCount != tt.expected.VideoCount {
				t.Errorf("expected VideoCount %s, got %s", tt.expected.VideoCount, got.VideoCount)
			}
			if tt.expected.Thumbnail != "" && got.Thumbnail != tt.expected.Thumbnail {
				t.Errorf("expected Thumbnail %s, got %s", tt.expected.Thumbnail, got.Thumbnail)
			}
		})
	}
}
