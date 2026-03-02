// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "typ-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test default config creation
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Errorf("Failed to load config: %v", err)
	}

	if cfg.PlaybackQuality != "1080p" {
		t.Errorf("Expected default quality 1080p, got %s", cfg.PlaybackQuality)
	}

	// Test persistence
	cfg.PlaybackQuality = "720p"
	data, _ := os.ReadFile(configPath) // Re-read to make sure it exists
	if len(data) == 0 {
		t.Error("Config file should have been created")
	}

	// Test manual modification
	err = os.WriteFile(configPath, []byte("playback_quality: 4k
language: jp"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg2, _ := LoadConfig(configPath)
	if cfg2.PlaybackQuality != "4k" {
		t.Errorf("Expected 4k, got %s", cfg2.PlaybackQuality)
	}
	if cfg2.Language != "jp" {
		t.Errorf("Expected jp, got %s", cfg2.Language)
	}
}
