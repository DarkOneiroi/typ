package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "typ-db-test")
	if err != nil { t.Fatal(err) }
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil { t.Fatalf("Failed to create DB: %v", err) }

	// Test History
	err = db.SaveWatchPosition("vid1", "Test Video", 10*time.Second, 100*time.Second)
	if err != nil { t.Errorf("SaveWatchPosition failed: %v", err) }

	h, _ := db.GetWatchHistory("vid1")
	if h == nil || h.LastPosition != 10*time.Second {
		t.Errorf("GetWatchHistory failed or returned wrong data")
	}

	// Test Download Queue
	err = db.AddToDownloadQueue("url1", "Title 1", "best", "/tmp", false)
	if err != nil { t.Errorf("AddToDownloadQueue failed: %v", err) }

	next, _ := db.GetNextDownload()
	if next == nil || next.URL != "url1" {
		t.Errorf("GetNextDownload failed")
	}

	db.UpdateDownloadStatus("url1", "downloading")
	db.UpdateAllDownloadingToPending()
	
	next2, _ := db.GetNextDownload()
	if next2 == nil || next2.URL != "url1" {
		t.Errorf("Resuming failed")
	}

	// Test Playlist
	item := struct{ ID, Title, URL, Duration string }{ID: "vid1", Title: "Title", URL: "url", Duration: "1:00"}
	db.AddToPlaylist("Default", item)
	
	list, _ := db.GetPlaylistItems("Default")
	if len(list) == 0 || list[0].ID != "vid1" {
		t.Errorf("Playlist retrieval failed")
	}

	db.RemoveFromPlaylist("Default", "vid1")
	list2, _ := db.GetPlaylistItems("Default")
	if len(list2) != 0 {
		t.Errorf("Playlist removal failed")
	}
}
