// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package downloader

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

// YtDlpDownloader implements the Downloader interface using yt-dlp.
type YtDlpDownloader struct {
	binPath string
}

// NewYtDlpDownloader creates a new YtDlpDownloader.
func NewYtDlpDownloader() (*YtDlpDownloader, error) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("yt-dlp not found in PATH: please install it (e.g., sudo pacman -S yt-dlp)")
	}
	return &YtDlpDownloader{binPath: path}, nil
}

func (y *YtDlpDownloader) Download(ctx context.Context, url string, quality Quality, outputDir string) (<-chan Progress, error) {
	progressChan := make(chan Progress)

	args := []string{
		"--newline",
		"--progress",
		"--progress-template", "downloading:%(progress._percent_str)s speed:%(progress._speed_str)s eta:%(progress._eta_str)s index:%(progress.playlist_index)s total:%(playlist_count)s title:%(title)s",
		"-o", outputDir + "/%(title)s.%(ext)s",
		"--yes-playlist", 
	}

	switch quality {
	case QualityAudio:
		args = append(args, "-x", "--audio-format", "mp3")
	case Quality1080p:
		args = append(args, "-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]")
	case Quality720p:
		args = append(args, "-f", "bestvideo[height<=720]+bestaudio/best[height<=720]")
	default:
		args = append(args, "-f", "bestvideo+bestaudio/best")
	}

	args = append(args, url)

	cmd := exec.CommandContext(ctx, y.binPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		defer close(progressChan)
		scanner := bufio.NewScanner(stdout)
		// Improved regex to handle NA, playlist_count, and varied whitespace
		re := regexp.MustCompile(`downloading:\s*([\d.]+)%\s*speed:\s*(\S+)\s*eta:\s*(\S+)\s*index:\s*(\d+|NA)\s*total:\s*(\d+|NA|)\s*title:(.*)`)

		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindStringSubmatch(line)
			if len(matches) == 7 {
				percent, _ := strconv.ParseFloat(matches[1], 64)
				
				idx := 0
				if matches[4] != "NA" && matches[4] != "" { idx, _ = strconv.Atoi(matches[4]) }
				
				tot := 0
				if matches[5] != "NA" && matches[5] != "" { tot, _ = strconv.Atoi(matches[5]) }

				progressChan <- Progress{
					Percentage:        percent,
					Speed:             matches[2],
					ETA:               matches[3],
					PlaylistIndex:     idx,
					PlaylistTotal:     tot,
					CurrentVideoTitle: matches[6],
				}
			}
		}
		_ = cmd.Wait()
	}()

	return progressChan, nil
}

func (y *YtDlpDownloader) GetFormats(ctx context.Context, url string) ([]string, error) {
	return nil, nil // Not used for now
}
