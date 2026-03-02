package downloader

import (
	"context"
)

// Quality represents the desired download quality.
type Quality string

const (
	QualityBest  Quality = "best"
	Quality1080p Quality = "1080p"
	Quality720p  Quality = "720p"
	QualityAudio Quality = "audio_only"
)

// Progress indicates the current state of a download.
type Progress struct {
	Percentage        float64
	Speed             string
	ETA               string
	Filename          string
	Title             string
	CurrentVideoTitle string
	URL               string
	IsChannel         bool
	PlaylistIndex     int
	PlaylistTotal     int
	ChannelVideos     []string
	}

// Downloader defines the contract for downloading media.
// Implementations can wrap yt-dlp, use internal Go libraries, or other tools.
type Downloader interface {
	// Download downloads a video or audio stream from the given URL.
	// It returns a channel that emits progress updates and an error if the download fails to start.
	Download(ctx context.Context, url string, quality Quality, outputDir string) (<-chan Progress, error)

	// GetFormats retrieves available formats for a video without downloading.
	GetFormats(ctx context.Context, url string) ([]string, error)
}
