# TYP: Terminal YouTube Player - Context & Architecture

This file outlines the current architecture and state of the `typ` project as of February 26, 2026.

## Project Overview
`typ` is a high-density Terminal User Interface (TUI) YouTube player and downloader. It follows a client-daemon architecture to provide persistent background operations.

## Core Architecture

### 1. Client-Daemon Model
- **Daemon (`internal/ui/daemon.go`)**: Coordinates background tasks. It manages an `mpv` instance for playback and a sequential worker for downloads.
- **TUI (`internal/ui/model.go`)**: A rich Bubble Tea interface. It connects to the daemon via Unix Domain Sockets (`internal/ipc`).
- **Persistence**: All state (history, downloads, playlists) is stored in a local SQLite database (`internal/state/db.go`).

### 2. Playback Engine
- **Backend**: `mpv` via JSON-IPC.
- **Optimizations**: Uses advanced flags (`--cache`, `--demuxer-max-bytes`, `--ytdl-raw-options`) to minimize buffering and start videos instantly.
- **Tracking**: Polls `mpv` every second to save watch progress to SQLite.

### 3. Sequential Download Worker
- **Queue**: Downloads are queued in SQLite. The daemon processes them one-by-one to prevent network congestion.
- **Features**: Support for pausing/resuming the worker. Persistence ensures downloads resume after a restart.
- **Organization**: Channels are automatically downloaded into their own dedicated subfolders.

### 4. Rich TUI Views
1. **Search**: Select between Video and Channel search.
2. **Results**: Browse rich, multi-line results with thumbnails, views, and upload dates.
3. **Downloads**: Monitor real-time progress of all queued and active downloads.
4. **History**: View and replay your watch history.
5. **Playlist**: Manage custom, named playlists.

### 5. Graphics Support
- **Kitty Protocol**: Supports "real" image rendering in Kitty, WezTerm, and Ghostty.
- **Fallback**: High-resolution Unicode half-blocks (`▀`) for other terminals.
- **Lazy Loading**: Thumbnails load after a 300ms debounce to ensure smooth scrolling.

## Key Components

- `internal/auth`: Browser cookie integration for authenticated access.
- `internal/config`: YAML-based configuration (`~/.config/typ/config.yaml`).
- `internal/downloader`: Wrapper around `yt-dlp` with detailed progress parsing.
- `internal/search`: Advanced searcher utilizing filtered YouTube URLs for accurate result types.

## Development Standards
- **Modularity**: All major engines are defined by interfaces.
- **Stability**: Sequential worker and SQLite persistence ensure data integrity.
- **Installation**: Unified `make install` handles builds and process management.
