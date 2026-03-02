package search

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ytDlpEntry represents the structured JSON output from yt-dlp.
// Using typed structs instead of maps ensures type safety and better performance.
type ytDlpEntry struct {
	ID               string           `json:"id"`
	Type             string           `json:"_type"`
	IeKey            string           `json:"ie_key"`
	Title            string           `json:"title"`
	Uploader         string           `json:"uploader"`
	Channel          string           `json:"channel"`
	URL              string           `json:"url"`
	WebpageURL       string           `json:"webpage_url"`
	Duration         float64          `json:"duration"`
	ViewCount        float64          `json:"view_count"`
	UploadDate       string           `json:"upload_date"`
	Description      string           `json:"description"`
	Thumbnail        string           `json:"thumbnail"`
	Thumbnails       []ytDlpThumbnail `json:"thumbnails"`
	VideoCount       float64          `json:"video_count"`
	PlaylistCount    float64          `json:"playlist_count"`
	SubscriberCount  float64          `json:"subscriber_count"`
}

type ytDlpThumbnail struct {
	URL    string  `json:"url"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// YtDlpSearcher implements the Searcher interface using yt-dlp.
type YtDlpSearcher struct {
	binPath string
}

func NewYtDlpSearcher() (*YtDlpSearcher, error) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("search: yt-dlp not found in PATH: %w", err)
	}
	return &YtDlpSearcher{binPath: path}, nil
}

func (y *YtDlpSearcher) Search(ctx context.Context, query string, resultType ResultType, page int) (<-chan Result, error) {
	var searchQuery string
	perPage := 20
	
	if resultType == ResultTypeChannel {
		searchQuery = fmt.Sprintf("https://www.youtube.com/results?search_query=%s&sp=EgIQAg%%3D%%3D", query)
	} else {
		start := (page - 1) * perPage
		searchQuery = fmt.Sprintf("ytsearch%d:%s", start+perPage, query)
	}

	args := []string{
		"--dump-json",
		"--flat-playlist",
		"--no-playlist",
		searchQuery,
	}

	cmd := exec.CommandContext(ctx, y.binPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("search: failed to create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("search: failed to start yt-dlp: %w", err)
	}

	resultsChan := make(chan Result)
	go func() {
		defer close(resultsChan)
		scanner := bufio.NewScanner(stdout)
		count := 0
		start := (page - 1) * perPage

		for scanner.Scan() {
			var entry ytDlpEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				continue
			}

			resType := ResultTypeVideo
			if entry.Type == "playlist" {
				resType = ResultTypePlaylist
			} else if entry.Type == "url" {
				// Detect Channel based on URL patterns
				if strings.Contains(entry.URL, "/channel/") || strings.Contains(entry.URL, "/@") || 
				   strings.Contains(entry.WebpageURL, "/channel/") || strings.Contains(entry.WebpageURL, "/@") {
					resType = ResultTypeChannel
				} else if entry.IeKey == "YoutubeTab" {
					resType = ResultTypePlaylist
				}
			}

			// Filter based on requested type
			if resultType == ResultTypeChannel && resType != ResultTypeChannel { continue }
			if resultType == ResultTypeVideo && resType != ResultTypeVideo { continue }

			// Pagination logic for non-channel results (handled by ytsearch count mostly)
			if resultType != ResultTypeChannel {
				if count < start { count++; continue }
				if count >= start+perPage { break }
				count++
			}

			// Map to internal Result type
			resultsChan <- y.mapToResult(entry, resType)
		}
		_ = cmd.Wait()
	}()

	return resultsChan, nil
}

func (y *YtDlpSearcher) mapToResult(entry ytDlpEntry, resType ResultType) Result {
	uploader := entry.Uploader
	if uploader == "" { uploader = entry.Channel }
	if uploader == "" { uploader = "Unknown Channel" }

	url := entry.URL
	if !strings.HasPrefix(url, "http") {
		url = entry.WebpageURL
	}

	res := Result{
		ID:          entry.ID,
		Type:        resType,
		Title:       entry.Title,
		ChannelName: uploader,
		URL:         url,
		Description: entry.Description,
	}

	// Format Duration or Video Count
	if resType == ResultTypeChannel {
		res.VideoCount = y.formatChannelStats(entry)
	} else {
		res.Duration = y.formatDuration(entry.Duration)
	}

	// Format Views
	if entry.ViewCount > 0 {
		res.Views = y.formatLargeNumber(entry.ViewCount) + " views"
	}

	// Format Upload Date (YYYYMMDD -> YYYY-MM-DD)
	if len(entry.UploadDate) == 8 {
		res.UploadDate = fmt.Sprintf("%s-%s-%s", entry.UploadDate[:4], entry.UploadDate[4:6], entry.UploadDate[6:8])
	}

	// Best Thumbnail logic
	res.Thumbnail = entry.Thumbnail
	if res.Thumbnail == "" && len(entry.Thumbnails) > 0 {
		maxRes := 0.0
		for _, t := range entry.Thumbnails {
			if t.Width*t.Height >= maxRes {
				maxRes = t.Width * t.Height
				res.Thumbnail = t.URL
			}
		}
	}
	if strings.HasPrefix(res.Thumbnail, "//") {
		res.Thumbnail = "https:" + res.Thumbnail
	}

	return res
}

func (y *YtDlpSearcher) formatDuration(d float64) string {
	h := int(d) / 3600
	m := (int(d) % 3600) / 60
	s := int(d) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (y *YtDlpSearcher) formatChannelStats(entry ytDlpEntry) string {
	if entry.VideoCount > 0 { return fmt.Sprintf("%d items", int(entry.VideoCount)) }
	if entry.PlaylistCount > 0 { return fmt.Sprintf("%d items", int(entry.PlaylistCount)) }
	if entry.SubscriberCount > 0 { return y.formatLargeNumber(entry.SubscriberCount) + " subs" }
	return "Channel"
}

func (y *YtDlpSearcher) formatLargeNumber(n float64) string {
	if n > 1000000 { return fmt.Sprintf("%.1fM", n/1000000.0) }
	if n > 1000 { return fmt.Sprintf("%.1fK", n/1000.0) }
	return fmt.Sprintf("%d", int(n))
}

func (y *YtDlpSearcher) Suggest(ctx context.Context, query string) ([]string, error) {
	return nil, nil
}
