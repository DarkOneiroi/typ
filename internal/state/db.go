// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package state

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type WatchHistory struct {
	VideoID      string
	Title        string
	LastPosition time.Duration
	Duration     time.Duration
	UpdatedAt    time.Time
}

type DB struct {
	conn *sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS watch_history (
			video_id TEXT PRIMARY KEY,
			title TEXT,
			last_position INTEGER,
			duration INTEGER,
			updated_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS downloads (
			video_id TEXT PRIMARY KEY,
			file_path TEXT,
			downloaded_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE
		);
		CREATE TABLE IF NOT EXISTS playlist_items (
			playlist_id INTEGER,
			video_id TEXT,
			title TEXT,
			url TEXT,
			duration TEXT,
			added_at DATETIME,
			PRIMARY KEY (playlist_id, video_id),
			FOREIGN KEY(playlist_id) REFERENCES playlists(id)
		);
		CREATE TABLE IF NOT EXISTS download_queue (
			url TEXT PRIMARY KEY,
			title TEXT,
			quality TEXT,
			target_dir TEXT,
			is_channel BOOLEAN,
			status TEXT,
			added_at DATETIME
		);
		INSERT OR IGNORE INTO playlists (name) VALUES ('Default');
	`)
	if err != nil {
		return nil, err
	}

	return &DB{conn: db}, nil
}

func (d *DB) AddToDownloadQueue(url, title, quality, targetDir string, isChannel bool) error {
	_, err := d.conn.Exec(`
		INSERT INTO download_queue (url, title, quality, target_dir, is_channel, status, added_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?)
		ON CONFLICT(url) DO NOTHING
	`, url, title, quality, targetDir, isChannel, time.Now())
	return err
}

func (d *DB) GetNextDownload() (*struct{ URL, Title, Quality, TargetDir string; IsChannel bool }, error) {
	return d.GetNextDownloadExcluding(nil)
}

func (d *DB) GetNextDownloadExcluding(excluding map[string]bool) (*struct{ URL, Title, Quality, TargetDir string; IsChannel bool }, error) {
	query := "SELECT url, title, quality, target_dir, is_channel FROM download_queue WHERE status = 'pending'"
	var args []interface{}
	if len(excluding) > 0 {
		placeholders := []string{}
		for url := range excluding {
			placeholders = append(placeholders, "?")
			args = append(args, url)
		}
		query += " AND url NOT IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY added_at ASC LIMIT 1"

	row := d.conn.QueryRow(query, args...)
	var item struct{ URL, Title, Quality, TargetDir string; IsChannel bool }
	err := row.Scan(&item.URL, &item.Title, &item.Quality, &item.TargetDir, &item.IsChannel)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &item, err
}

func (d *DB) UpdateDownloadStatus(url, status string) error {
	_, err := d.conn.Exec("UPDATE download_queue SET status = ? WHERE url = ?", status, url)
	return err
}

func (d *DB) UpdateAllDownloadingToPending() error {
	_, err := d.conn.Exec("UPDATE download_queue SET status = 'pending' WHERE status = 'downloading'")
	return err
}

func (d *DB) GetPendingDownloads() ([]struct{ URL, Title, Quality, TargetDir string; IsChannel bool }, error) {
	rows, err := d.conn.Query("SELECT url, title, quality, target_dir, is_channel FROM download_queue WHERE status = 'pending'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []struct{ URL, Title, Quality, TargetDir string; IsChannel bool }
	for rows.Next() {
		var item struct{ URL, Title, Quality, TargetDir string; IsChannel bool }
		if err := rows.Scan(&item.URL, &item.Title, &item.Quality, &item.TargetDir, &item.IsChannel); err == nil {
			list = append(list, item)
		}
	}
	return list, nil
}

func (d *DB) RemoveFromDownloadQueue(url string) error {
	_, err := d.conn.Exec("DELETE FROM download_queue WHERE url = ?", url)
	return err
}

func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.conn.Exec(query, args...)
}

func (d *DB) AddToPlaylist(playlistName string, item struct{ ID, Title, URL, Duration string }) error {
	var pid int
	err := d.conn.QueryRow("SELECT id FROM playlists WHERE name = ?", playlistName).Scan(&pid)
	if err != nil {
		return err
	}

	_, err = d.conn.Exec(`
		INSERT INTO playlist_items (playlist_id, video_id, title, url, duration, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(playlist_id, video_id) DO NOTHING
	`, pid, item.ID, item.Title, item.URL, item.Duration, time.Now())
	return err
}

func (d *DB) RemoveFromPlaylist(playlistName, videoID string) error {
	var pid int
	err := d.conn.QueryRow("SELECT id FROM playlists WHERE name = ?", playlistName).Scan(&pid)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec("DELETE FROM playlist_items WHERE playlist_id = ? AND video_id = ?", pid, videoID)
	return err
}

func (d *DB) ListPlaylists() ([]string, error) {
	rows, err := d.conn.Query("SELECT name FROM playlists ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			names = append(names, name)
		}
	}
	return names, nil
}

func (d *DB) CreatePlaylist(name string) error {
	_, err := d.conn.Exec("INSERT OR IGNORE INTO playlists (name) VALUES (?)", name)
	return err
}

func (d *DB) GetPlaylistItems(playlistName string) ([]struct{ ID, Title, URL, Duration string }, error) {
	rows, err := d.conn.Query(`
		SELECT video_id, title, url, duration 
		FROM playlist_items pi
		JOIN playlists p ON pi.playlist_id = p.id
		WHERE p.name = ?
		ORDER BY added_at DESC
	`, playlistName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []struct{ ID, Title, URL, Duration string }
	for rows.Next() {
		var item struct{ ID, Title, URL, Duration string }
		if err := rows.Scan(&item.ID, &item.Title, &item.URL, &item.Duration); err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func (d *DB) GetHistory() ([]WatchHistory, error) {
	rows, err := d.conn.Query("SELECT video_id, title, last_position, duration, updated_at FROM watch_history ORDER BY updated_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []WatchHistory
	for rows.Next() {
		var h WatchHistory
		var lastPos, dur int64
		if err := rows.Scan(&h.VideoID, &h.Title, &lastPos, &dur, &h.UpdatedAt); err == nil {
			h.LastPosition = time.Duration(lastPos)
			h.Duration = time.Duration(dur)
			history = append(history, h)
		}
	}
	return history, nil
}

func (d *DB) SaveWatchPosition(videoID, title string, pos, duration time.Duration) error {
	_, err := d.conn.Exec(`
		INSERT INTO watch_history (video_id, title, last_position, duration, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			last_position = excluded.last_position,
			updated_at = excluded.updated_at
	`, videoID, title, int64(pos), int64(duration), time.Now())
	return err
}

func (d *DB) GetWatchHistory(videoID string) (*WatchHistory, error) {
	row := d.conn.QueryRow("SELECT video_id, title, last_position, duration, updated_at FROM watch_history WHERE video_id = ?", videoID)
	var h WatchHistory
	var lastPos, dur int64
	err := row.Scan(&h.VideoID, &h.Title, &lastPos, &dur, &h.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	h.LastPosition = time.Duration(lastPos)
	h.Duration = time.Duration(dur)
	return &h, nil
}

func (d *DB) MarkAsDownloaded(videoID, filePath string) error {
	_, err := d.conn.Exec("INSERT INTO downloads (video_id, file_path, downloaded_at) VALUES (?, ?, ?)", videoID, filePath, time.Now())
	return err
}

func (d *DB) IsDownloaded(videoID string) (bool, string) {
	var path string
	err := d.conn.QueryRow("SELECT file_path FROM downloads WHERE video_id = ?", videoID).Scan(&path)
	if err == nil {
		// Verify file still exists
		if _, err := os.Stat(path); err == nil {
			return true, path
		}
	}
	return false, ""
}
