// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DownloadDir       string `yaml:"download_dir"`
	PlaybackQuality   string `yaml:"playback_quality"`
	ParallelDownloads int    `yaml:"parallel_downloads"`
	Language          string `yaml:"language"`
	Mpv               struct {
		CacheEnabled bool   `yaml:"cache_enabled"`
		CacheSize    string `yaml:"cache_size"`
		ExtraArgs    []string `yaml:"extra_args"`
	} `yaml:"mpv"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	c := &Config{
		DownloadDir:       filepath.Join(home, "Downloads", "typ"),
		PlaybackQuality:   "1080p",
		ParallelDownloads: 1,
		Language:          "en",
	}
	c.Mpv.CacheEnabled = true
	c.Mpv.CacheSize = "500M"
	c.Mpv.ExtraArgs = []string{}
	return c
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Save default if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return cfg, fmt.Errorf("failed to create config directory: %w", err)
		}
		data, _ := yaml.Marshal(cfg)
		if err := os.WriteFile(path, data, 0644); err != nil {
			return cfg, fmt.Errorf("failed to write default config: %w", err)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Validation / Sanitization
	if cfg.ParallelDownloads < 1 {
		cfg.ParallelDownloads = 1
	}

	return cfg, nil
}
