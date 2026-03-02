// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package player

import (
	"time"
)

// State represents the current playback state.
type State string

const (
	StatePlaying State = "playing"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
	StateIdle    State = "idle"
)

// Track represents a media item in the playlist.
type Track struct {
	ID       string
	Title    string
	Artist   string // Channel name
	Duration time.Duration
	URL      string
}

// Status contains the snapshot of the player's current status.
type Status struct {
	State        State
	CurrentTrack *Track
	Position     time.Duration
	Volume       int
	Muted        bool
}

// Player defines the contract for media playback.
// Implementations typically wrap mpv or other media engines.
type Player interface {
	// Load loads a track for playback. If autoPlay is true, it starts immediately.
	Load(track Track, autoPlay bool) error

	// Play resumes playback.
	Play() error

	// Pause pauses playback.
	Pause() error

	// TogglePause toggles between play and pause.
	TogglePause() error

	// Stop stops playback and clears the current track.
	Stop() error

	// Seek seeks to the specified position.
	Seek(position time.Duration) error

	// SetVolume sets the volume (0-100).
	SetVolume(volume int) error

	// GetStatus returns the current player status.
	GetStatus() (Status, error)

	// Close cleans up resources (e.g., stops the mpv process).
	Close() error
}
