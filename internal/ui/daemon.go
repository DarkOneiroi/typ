package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/darkone/typ/internal/config"
	"github.com/darkone/typ/internal/downloader"
	"github.com/darkone/typ/internal/ipc"
	"github.com/darkone/typ/internal/player"
	"github.com/darkone/typ/internal/search"
	"github.com/darkone/typ/internal/state"
)

// Daemon handles the background processes and coordinates components.
type Daemon struct {
	player        player.Player
	searcher      search.Searcher
	downloader    downloader.Downloader
	socketPath    string
	config        *config.Config
	db            *state.DB
	downloads     map[string]downloader.Progress
	downloadOrder []string
	channelVideos map[string][]string
	downloadsMu   sync.Mutex
	isPaused      bool
	currentCancel map[string]context.CancelFunc
	currentURLs   map[string]bool
	searchResults []search.Result
	searchMu      sync.Mutex
	isSearching   bool
}

func NewDaemon(socketPath string, cfg *config.Config) (*Daemon, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(filepath.Dir(socketPath), "typ.db")
	db, err := state.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}

	mpvSocket := socketPath + ".mpv"
	p, err := player.NewMpvPlayer(mpvSocket)
	if err != nil {
		return nil, fmt.Errorf("player error: %w", err)
	}

	s, err := search.NewYtDlpSearcher()
	if err != nil {
		return nil, fmt.Errorf("searcher error: %w", err)
	}

	d, err := downloader.NewYtDlpDownloader()
	if err != nil {
		return nil, fmt.Errorf("downloader error: %w", err)
	}

	return &Daemon{
		player:        p,
		searcher:      s,
		downloader:    d,
		socketPath:    socketPath,
		config:        cfg,
		db:            db,
		downloads:     make(map[string]downloader.Progress),
		downloadOrder: []string{},
		channelVideos: make(map[string][]string),
		currentCancel: make(map[string]context.CancelFunc),
		currentURLs:   make(map[string]bool),
	}, nil
}

func (d *Daemon) Start() error {
	go d.statusLoop()
	
	parallel := d.config.ParallelDownloads
	if parallel < 1 { parallel = 1 }
	for i := 0; i < parallel; i++ {
		go d.downloadWorker()
	}

	_ = d.db.UpdateAllDownloadingToPending()

	os.Remove(d.socketPath)
	l, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return err
	}
	defer l.Close()

	log.Printf("Daemon listening on %s", d.socketPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go d.handleConnection(conn)
	}
}

func (d *Daemon) downloadWorker() {
	for {
		if d.isPaused {
			time.Sleep(time.Second)
			continue
		}

		d.downloadsMu.Lock()
		item, err := d.db.GetNextDownloadExcluding(d.currentURLs)
		if err == nil && item != nil {
			d.currentURLs[item.URL] = true
		}
		d.downloadsMu.Unlock()

		if err != nil || item == nil {
			time.Sleep(time.Second)
			continue
		}

		_ = d.db.UpdateDownloadStatus(item.URL, "downloading")

		ctx, cancel := context.WithCancel(context.Background())
		d.downloadsMu.Lock()
		d.currentCancel[item.URL] = cancel
		d.downloadsMu.Unlock()

		d.executeDownload(ctx, item.URL, item.Title, downloader.Quality(item.Quality), item.TargetDir, item.IsChannel)
		
		d.downloadsMu.Lock()
		delete(d.currentCancel, item.URL)
		delete(d.currentURLs, item.URL)
		d.downloadsMu.Unlock()

		time.Sleep(500 * time.Millisecond)
	}
}

func (d *Daemon) executeDownload(ctx context.Context, url, title string, quality downloader.Quality, targetDir string, isChannel bool) {
	d.downloadsMu.Lock()
	if _, exists := d.downloads[url]; !exists {
		d.downloadOrder = append(d.downloadOrder, url)
	}
	d.downloadsMu.Unlock()

	progress, err := d.downloader.Download(ctx, url, quality, targetDir)
	if err != nil {
		log.Printf("Download error for %s: %v", title, err)
		return
	}

	var lastProgress downloader.Progress
	success := false
	for {
		select {
		case <-ctx.Done():
			// Marked as pending in ReqPause if needed
			return
		case p, ok := <-progress:
			if !ok {
				success = (lastProgress.Percentage >= 100)
				goto FINISH
			}
			p.Title = title
			p.URL = url
			p.IsChannel = isChannel

			d.downloadsMu.Lock()
			if isChannel && p.CurrentVideoTitle != "" {
				titles := d.channelVideos[url]
				already := false
				for _, t := range titles {
					if t == p.CurrentVideoTitle {
						already = true
						break
					}
				}
				if !already {
					d.channelVideos[url] = append(titles, p.CurrentVideoTitle)
				}
			}
			p.ChannelVideos = d.channelVideos[url]
			d.downloads[url] = p
			d.downloadsMu.Unlock()
			lastProgress = p
		}
	}

FINISH:
	d.downloadsMu.Lock()
	delete(d.channelVideos, url)
	d.downloadsMu.Unlock()

	if success {
		_ = d.db.RemoveFromDownloadQueue(url)
		if !isChannel && lastProgress.Filename != "" {
			absPath, _ := filepath.Abs(filepath.Join(targetDir, lastProgress.Filename))
			videoID := url
			if parts := strings.Split(url, "v="); len(parts) > 1 {
				videoID = parts[1]
			}
			_ = d.db.MarkAsDownloaded(videoID, absPath)
		}
	} else {
		// If it wasn't finished and not explicitly canceled, set back to pending
		_ = d.db.UpdateDownloadStatus(url, "pending")
	}
}

func (d *Daemon) statusLoop() {
	ticker := time.NewTicker(1 * time.Second)
	statusFile := filepath.Join(filepath.Dir(d.socketPath), "status.json")

	for range ticker.C {
		status, err := d.player.GetStatus()
		if err != nil {
			continue
		}

		var text string
		if status.CurrentTrack != nil {
			text = fmt.Sprintf("󰗃 %s", status.CurrentTrack.Title)
			_ = d.db.SaveWatchPosition(status.CurrentTrack.ID, status.CurrentTrack.Title, status.Position, status.CurrentTrack.Duration)
		} else {
			text = ""
		}

		waybarStatus := map[string]interface{}{
			"text":    text,
			"tooltip": text,
			"class":   string(status.State),
		}

		data, _ := json.Marshal(waybarStatus)
		_ = os.WriteFile(statusFile, data, 0644)
	}
}

func (d *Daemon) handleConnection(conn net.Conn) {
	defer conn.Close()
	var req ipc.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	resp := d.processRequest(req)
	json.NewEncoder(conn).Encode(resp)
}

func (d *Daemon) processRequest(req ipc.Request) ipc.Response {
	switch req.Type {
	case ipc.ReqStatus:
		status, err := d.player.GetStatus()
		if err != nil {
			return ipc.Response{Success: false, Error: err.Error()}
		}
		return ipc.Response{Success: true, Payload: map[string]interface{}{"status": status}}

	case ipc.ReqPlay:
		url, _ := req.Payload["url"].(string)
		title, _ := req.Payload["title"].(string)
		id, _ := req.Payload["id"].(string)
		err := d.player.Load(player.Track{URL: url, Title: title, ID: id}, true)
		if err != nil { return ipc.Response{Success: false, Error: err.Error()} }
		return ipc.Response{Success: true}

	case ipc.ReqPause:
		err := d.player.Pause()
		return ipc.Response{Success: err == nil, Error: fmt.Sprint(err)}

	case ipc.ReqStop:
		log.Println("Stopping daemon...")
		go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
		return ipc.Response{Success: true}

	case ipc.ReqSearch:
		query, _ := req.Payload["query"].(string)
		searchTypeStr, _ := req.Payload["search_type"].(string)
		searchType := search.ResultType(searchTypeStr)
		page, _ := req.Payload["page"].(float64)
		if page == 0 { page = 1 }

		d.searchMu.Lock()
		d.isSearching = true
		d.searchResults = []search.Result{}
		d.searchMu.Unlock()

		go func() {
			resultsChan, err := d.searcher.Search(context.Background(), query, searchType, int(page))
			if err != nil {
				d.searchMu.Lock()
				d.isSearching = false
				d.searchMu.Unlock()
				return
			}

			for res := range resultsChan {
				if isDownloaded, _ := d.db.IsDownloaded(res.ID); isDownloaded { res.IsDownloaded = true }
				if h, _ := d.db.GetWatchHistory(res.ID); h != nil {
					res.WatchedPos = fmt.Sprintf("%d:%02d / %s", int(h.LastPosition.Seconds())/60, int(h.LastPosition.Seconds())%60, res.Duration)
				}
				
				d.searchMu.Lock()
				d.searchResults = append(d.searchResults, res)
				d.searchMu.Unlock()
			}

			d.searchMu.Lock()
			d.isSearching = false
			d.searchMu.Unlock()
		}()
		return ipc.Response{Success: true}

	case ipc.ReqSearchStatus:
		d.searchMu.Lock()
		defer d.searchMu.Unlock()
		return ipc.Response{Success: true, Payload: map[string]interface{}{
			"results":      d.searchResults,
			"is_searching": d.isSearching,
		}}

	case ipc.ReqDownload:
		url, _ := req.Payload["url"].(string)
		title, _ := req.Payload["title"].(string)
		isChannel, _ := req.Payload["is_channel"].(bool)
		qualityStr, _ := req.Payload["quality"].(string)
		targetDir := d.config.DownloadDir
		if isChannel { targetDir = filepath.Join(targetDir, title) }
		_ = os.MkdirAll(targetDir, 0755)
		_ = d.db.AddToDownloadQueue(url, title, qualityStr, targetDir, isChannel)
		return ipc.Response{Success: true}

	case ipc.ReqDownloadsStatus:
		d.downloadsMu.Lock()
		defer d.downloadsMu.Unlock()
		orderedProgress := []downloader.Progress{}
		for _, url := range d.downloadOrder {
			if p, ok := d.downloads[url]; ok { orderedProgress = append(orderedProgress, p) }
		}
		// Also include ones that are in DB but not yet active in memory
		pending, _ := d.db.GetPendingDownloads()
		for _, p := range pending {
			alreadyIn := false
			for _, url := range d.downloadOrder { if url == p.URL { alreadyIn = true; break } }
			if !alreadyIn {
				orderedProgress = append(orderedProgress, downloader.Progress{
					Title: p.Title, URL: p.URL, Percentage: 0, IsChannel: p.IsChannel,
				})
			}
		}

		return ipc.Response{Success: true, Payload: map[string]interface{}{
			"downloads": orderedProgress,
			"is_paused": d.isPaused,
		}}

	case ipc.ReqHistory:
		history, err := d.db.GetHistory()
		if err != nil { return ipc.Response{Success: false, Error: err.Error()} }
		return ipc.Response{Success: true, Payload: map[string]interface{}{"history": history}}

	case ipc.ReqPlaylist:
		name, _ := req.Payload["name"].(string)
		if name == "" { name = "Default" }
		items, err := d.db.GetPlaylistItems(name)
		if err != nil { return ipc.Response{Success: false, Error: err.Error()} }
		return ipc.Response{Success: true, Payload: map[string]interface{}{"items": items}}

	case ipc.ReqAddToPlaylist:
		name, _ := req.Payload["name"].(string); if name == "" { name = "Default" }
		item := struct{ ID, Title, URL, Duration string }{
			ID: req.Payload["id"].(string), Title: req.Payload["title"].(string),
			URL: req.Payload["url"].(string), Duration: req.Payload["duration"].(string),
		}
		err := d.db.AddToPlaylist(name, item)
		if err != nil { return ipc.Response{Success: false, Error: err.Error()} }
		return ipc.Response{Success: true}

	case ipc.ReqRemoveFromPlaylist:
		name, _ := req.Payload["name"].(string); if name == "" { name = "Default" }
		err := d.db.RemoveFromPlaylist(name, req.Payload["id"].(string))
		if err != nil { return ipc.Response{Success: false, Error: err.Error()} }
		return ipc.Response{Success: true}

	case ipc.ReqPauseDownload:
		d.isPaused = true
		d.downloadsMu.Lock()
		for _, cancel := range d.currentCancel { if cancel != nil { cancel() } }
		d.downloadsMu.Unlock()
		_ = d.db.UpdateAllDownloadingToPending()
		return ipc.Response{Success: true}

	case ipc.ReqResumeDownload:
		d.isPaused = false
		return ipc.Response{Success: true}

	case ipc.ReqCancelDownload:
		url, _ := req.Payload["url"].(string)
		d.downloadsMu.Lock()
		if cancel, ok := d.currentCancel[url]; ok && cancel != nil {
			cancel()
		}
		d.downloadsMu.Unlock()
		_ = d.db.RemoveFromDownloadQueue(url)
		d.downloadsMu.Lock()
		delete(d.downloads, url)
		delete(d.channelVideos, url)
		// Remove from order slice too
		newOrder := []string{}
		for _, u := range d.downloadOrder { if u != url { newOrder = append(newOrder, u) } }
		d.downloadOrder = newOrder
		d.downloadsMu.Unlock()
		return ipc.Response{Success: true}

	default:
		return ipc.Response{Success: false, Error: "unknown request type"}
	}
}
