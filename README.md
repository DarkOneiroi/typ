# TYP: Terminal YouTube Player

`typ` is a sophisticated, high-density Terminal YouTube Player and downloader. It features a persistent background daemon, SQLite-backed state management, and support for high-resolution graphics.

## Features
- **TUI**: Beautiful interface with dedicated views for Search, Results, Downloads, History, and Playlists.
- **Persistence**: All downloads, history, and playlists are saved in a local SQLite database.
- **Sequential Downloads**: A background worker processes downloads one-by-one to ensure stability.
- **Graphics**: Real image rendering using Kitty Graphics Protocol (supported in Kitty, WezTerm, Ghostty) with Unicode fallback.
- **Auto-Daemon**: The playback and download service starts automatically when you launch the TUI.
- **Waybar Integration**: Real-time playback status exported to `$XDG_RUNTIME_DIR/typ/status.json`.

## Prerequisites
- **Go** 1.21+
- **mpv** (with `yt-dlp` hook support)
- **yt-dlp** (installed and in PATH)

## Installation
```bash
git clone https://github.com/darkone/typ.git
cd typ
make install
```

## Keybindings

### Global
- `1-5`: Switch views (Search, Results, Downloads, History, Playlist)
- `q` / `ctrl+c`: Quit (asks for confirmation if downloads active)
- `space`: Toggle playback pause

### Search (1)
- `up/down`: Select search type (Videos/Channels)
- `enter`: Start typing search query

### Results (2)
- `j/k`: Navigate results
- `enter`: Play selected video
- `d`: Open download quality menu
- `a`: Add video to default playlist
- `n/p`: Next/Previous page of results

### Downloads (3)
- `p`: Pause download worker
- `r`: Resume download worker

### Playlist (5)
- `x`: Remove item from playlist
- `enter`: Play item

## Configuration
Config is stored in `~/.config/typ/config.yaml`. You can customize:
- `download_dir`: Base folder for all downloads.
- `playback_quality`: Default resolution for `mpv`.
- `mpv`: Cache settings and extra arguments.

## Architecture
See [CONTEXT.md](./CONTEXT.md) for a deep dive into the technical design.
