// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/darkone/typ/internal/downloader"
	"github.com/darkone/typ/internal/ipc"
	"github.com/darkone/typ/internal/player"
	"github.com/darkone/typ/internal/search"
	"github.com/darkone/typ/internal/state"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewMode int

const (
	SearchView ViewMode = iota
	ResultsView
	DownloadsView
	HistoryView
	PlaylistView
	DownloadQualityView
)

type searchMsg struct {
	results   []search.Result
	err       error
	isLoading bool
}

type historyMsg []state.WatchHistory
type playlistMsg []struct{ ID, Title, URL, Duration string }
type downloadsStatus struct {
	list     []downloader.Progress
	isPaused bool
}
type tickMsg time.Time

type Model struct {
	client           *ipc.Client
	viewMode         ViewMode
	query            string
	results          []search.Result
	history          []state.WatchHistory
	playlist         []struct{ ID, Title, URL, Duration string }
	cursor           int
	status           string
	err              error
	width            int
	height           int
	isSearching      bool
	isLoadingResults bool
	searchType       search.ResultType
	searchCursor     int
	downloadCursor   int
	qualities        []string
	downloads        []downloader.Progress
	isWorkerPaused   bool
	page             int
	isConfirmingQuit bool
	styles           Styles
}

func NewModel(client *ipc.Client) Model {
	return Model{
		client:           client,
		viewMode:         SearchView,
		status:           "Ready",
		qualities:        []string{"Best", "1080p", "720p", "Audio Only"},
		downloads:        []downloader.Progress{},
		searchType:       search.ResultTypeVideo,
		page:             1,
		styles:           DefaultStyles(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.tick()
}

func (m Model) tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.isConfirmingQuit {
			switch msg.String() {
			case "y", "Y": return m, tea.Quit
			case "n", "N", "esc": m.isConfirmingQuit = false; return m, nil
			}
			return m, nil
		}

		if m.isSearching {
			switch msg.String() {
			case "enter":
				m.isSearching = false
				m.isLoadingResults = true
				m.viewMode = ResultsView
				return m, m.performSearch()
			case "backspace":
				if len(m.query) > 0 { m.query = m.query[:len(m.query)-1] }
			case "esc": m.isSearching = false
			case "ctrl+c": return m, tea.Quit
			default:
				if len(msg.String()) == 1 { m.query += msg.String() }
			}
			return m, nil
		}

		// Global View Switching
		switch msg.String() {
		case "1": m.viewMode = SearchView; m.cursor = 0
		case "2": m.viewMode = ResultsView; m.cursor = 0
		case "3": m.viewMode = DownloadsView; m.cursor = 0
		case "4": m.viewMode = HistoryView; m.cursor = 0; return m, m.loadHistory()
		case "5": m.viewMode = PlaylistView; m.cursor = 0; return m, m.loadPlaylist()
		case "q", "ctrl+c":
			hasActive := false
			for _, d := range m.downloads {
				if d.Percentage > 0 && d.Percentage < 100 { hasActive = true; break }
			}
			if hasActive { m.isConfirmingQuit = true; return m, nil }
			return m, tea.Quit
		}

		if m.viewMode == SearchView {
			switch msg.String() {
			case "up", "k": if m.searchCursor > 0 { m.searchCursor-- }
			case "down", "j": if m.searchCursor < 1 { m.searchCursor++ }
			case "enter":
				if m.searchCursor == 0 { m.searchType = search.ResultTypeVideo } else { m.searchType = search.ResultTypeChannel }
				m.isSearching = true
				m.query = ""
			}
			return m, nil
		}

		if m.viewMode == DownloadQualityView {
			switch msg.String() {
			case "up", "k": if m.downloadCursor > 0 { m.downloadCursor-- }
			case "down", "j": if m.downloadCursor < len(m.qualities)-1 { m.downloadCursor++ }
			case "enter":
				m.viewMode = ResultsView
				m.status = "Download queued..."
				return m, m.performDownload(m.results[m.cursor], m.qualities[m.downloadCursor])
			case "esc", "q": m.viewMode = ResultsView
			}
			return m, nil
		}

		switch msg.String() {
		case "x":
			if m.viewMode == PlaylistView && len(m.playlist) > 0 {
				return m, m.removeFromPlaylist(m.playlist[m.cursor].ID)
			}
			if m.viewMode == DownloadsView && len(m.downloads) > 0 {
				return m, m.cancelDownload(m.cursor)
			}

		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			currListLen := 0
			switch m.viewMode {
			case ResultsView: currListLen = len(m.results)
			case HistoryView: currListLen = len(m.history)
			case PlaylistView: currListLen = len(m.playlist)
			case DownloadsView: currListLen = len(m.downloads)
			}
			if m.cursor < currListLen-1 { m.cursor++ }

		case "d":
			if m.viewMode == ResultsView && len(m.results) > 0 {
				m.viewMode = DownloadQualityView
				m.downloadCursor = 0
			}
		case "a":
			if m.viewMode == ResultsView && len(m.results) > 0 {
				return m, m.addToPlaylist(m.results[m.cursor])
			}

		case "n":
			if m.viewMode == ResultsView { m.page++; m.isLoadingResults = true; return m, m.performSearch() }
		case "p":
			if m.viewMode == ResultsView && m.page > 1 {
				m.page--
				m.isLoadingResults = true
				return m, m.performSearch()
			}
			if m.viewMode == DownloadsView { return m, m.controlDownload(ipc.ReqPauseDownload) }
		case "r":
			if m.viewMode == DownloadsView { return m, m.controlDownload(ipc.ReqResumeDownload) }

		case "enter":
			switch m.viewMode {
			case ResultsView: if len(m.results) > 0 { m.status = "Loading..."; return m, m.playVideo(m.results[m.cursor]) }
			case HistoryView:
				if len(m.history) > 0 {
					h := m.history[m.cursor]; m.status = "Loading..."
					return m, m.playTrack(player.Track{ID: h.VideoID, Title: h.Title, URL: "https://www.youtube.com/watch?v="+h.VideoID})
				}
			case PlaylistView:
				if len(m.playlist) > 0 {
					p := m.playlist[m.cursor]; m.status = "Loading..."
					return m, m.playTrack(player.Track{ID: p.ID, Title: p.Title, URL: p.URL})
				}
			}
		case " ": return m, m.togglePause()
		}

	case searchMsg:
		m.results = msg.results
		m.isLoadingResults = msg.isLoading
		if msg.err != nil {
			m.err = msg.err
			m.isLoadingResults = false
			return m, nil
		}
		if m.isLoadingResults {
			return m, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg { return m.updateSearchStatus()() })
		}
		if m.cursor >= len(m.results) { m.cursor = 0 }
		return m, nil

	case historyMsg: m.history = msg; return m, nil
	case playlistMsg: m.playlist = msg; return m, nil
	case tickMsg: return m, tea.Batch(m.tick(), m.updateDownloads())
	case downloadsStatus:
		m.downloads = msg.list
		m.isWorkerPaused = msg.isPaused
		return m, nil
	}

	return m, nil
}

func (m Model) loadHistory() tea.Cmd {
	return func() tea.Msg {
		resp, _ := m.client.SendRequest(ipc.Request{Type: ipc.ReqHistory})
		if resp != nil && resp.Success {
			var h []state.WatchHistory
			b, _ := json.Marshal(resp.Payload["history"]); json.Unmarshal(b, &h)
			return historyMsg(h)
		}
		return historyMsg{}
	}
}

func (m Model) loadPlaylist() tea.Cmd {
	return func() tea.Msg {
		resp, _ := m.client.SendRequest(ipc.Request{Type: ipc.ReqPlaylist, Payload: map[string]interface{}{"name": "Default"}})
		if resp != nil && resp.Success {
			var items []struct{ ID, Title, URL, Duration string }
			b, _ := json.Marshal(resp.Payload["items"]); json.Unmarshal(b, &items)
			return playlistMsg(items)
		}
		return playlistMsg{}
	}
}

func (m Model) addToPlaylist(res search.Result) tea.Cmd {
	return func() tea.Msg {
		m.client.SendRequest(ipc.Request{
			Type: ipc.ReqAddToPlaylist,
			Payload: map[string]interface{}{
				"id": res.ID, "title": res.Title, "url": res.URL, "duration": res.Duration,
			},
		})
		return nil
	}
}

func (m Model) removeFromPlaylist(id string) tea.Cmd {
	return func() tea.Msg {
		m.client.SendRequest(ipc.Request{Type: ipc.ReqRemoveFromPlaylist, Payload: map[string]interface{}{"id": id}})
		return m.loadPlaylist()()
	}
}

func (m Model) updateDownloads() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.SendRequest(ipc.Request{Type: ipc.ReqDownloadsStatus})
		if err != nil || !resp.Success { return nil }
		data, _ := resp.Payload["downloads"].([]interface{})
		var downloads []downloader.Progress
		for _, v := range data {
			b, _ := json.Marshal(v); var p downloader.Progress
			json.Unmarshal(b, &p); downloads = append(downloads, p)
		}
		isPaused, _ := resp.Payload["is_paused"].(bool)
		return downloadsStatus{list: downloads, isPaused: isPaused}
	}
}

func (m Model) performSearch() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.SendRequest(ipc.Request{
			Type:    ipc.ReqSearch,
			Payload: map[string]interface{}{"query": m.query, "search_type": string(m.searchType), "page": m.page},
		})
		if err != nil { return searchMsg{err: err} }
		if !resp.Success { return searchMsg{err: fmt.Errorf("%s", resp.Error)} }
		return searchMsg{results: []search.Result{}, isLoading: true}
	}
}

func (m Model) updateSearchStatus() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.SendRequest(ipc.Request{Type: ipc.ReqSearchStatus})
		if err != nil || !resp.Success { return searchMsg{isLoading: false} }
		data, _ := resp.Payload["results"].([]interface{})
		var results []search.Result
		for _, d := range data {
			b, _ := json.Marshal(d); var r search.Result
			if err := json.Unmarshal(b, &r); err == nil { results = append(results, r) }
		}
		isSearching, _ := resp.Payload["is_searching"].(bool)
		return searchMsg{results: results, isLoading: isSearching}
	}
}

func (m Model) playVideo(res search.Result) tea.Cmd {
	return m.playTrack(player.Track{ID: res.ID, Title: res.Title, URL: res.URL})
}

func (m Model) playTrack(t player.Track) tea.Cmd {
	return func() tea.Msg {
		m.client.SendRequest(ipc.Request{
			Type: ipc.ReqPlay,
			Payload: map[string]interface{}{"url": t.URL, "title": t.Title, "id": t.ID},
		})
		return nil
	}
}

func (m Model) performDownload(res search.Result, quality string) tea.Cmd {
	return func() tea.Msg {
		qMap := map[string]string{"Best": "best", "1080p": "1080p", "720p": "720p", "Audio Only": "audio_only"}
		m.client.SendRequest(ipc.Request{
			Type: ipc.ReqDownload,
			Payload: map[string]interface{}{
				"url": res.URL, "title": res.Title, "quality": qMap[quality], "is_channel": res.Type == search.ResultTypeChannel,
			},
		})
		return nil
	}
}

func (m Model) togglePause() tea.Cmd {
	return func() tea.Msg { m.client.SendRequest(ipc.Request{Type: ipc.ReqPause}); return nil }
}

func (m Model) controlDownload(reqType ipc.RequestType) tea.Cmd {
	return func() tea.Msg { m.client.SendRequest(ipc.Request{Type: reqType}); return nil }
}

func (m Model) cancelDownload(index int) tea.Cmd {
	if index < 0 || index >= len(m.downloads) { return nil }
	url := m.downloads[index].URL
	return func() tea.Msg {
		m.client.SendRequest(ipc.Request{
			Type: ipc.ReqCancelDownload,
			Payload: map[string]interface{}{"url": url},
		})
		return nil
	}
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	if m.isConfirmingQuit {
		return m.styles.Confirmation.
			Width(m.width).
			Height(m.height).
			Render("Active downloads detected!
Quit and resume later? (y/n)")
	}

	header := m.renderHeader()
	legend := m.renderLegend()
	
	mainHeight := m.height - lipgloss.Height(header) - lipgloss.Height(legend) - 1
	if mainHeight < 1 {
		mainHeight = 1
	}

	var content string
	switch m.viewMode {
	case SearchView:
		content = m.renderSearchView()
	case ResultsView:
		content = m.renderResultsView(mainHeight)
	case DownloadsView:
		content = m.renderDownloadsView()
	case HistoryView:
		content = m.renderHistoryView()
	case PlaylistView:
		content = m.renderPlaylistView()
	case DownloadQualityView:
		content = m.renderDownloadQualityView()
	}

	mainContent := lipgloss.NewStyle().Height(mainHeight).MaxHeight(mainHeight).Render(content)
	
	return header + mainContent + "
" + legend
}

func (m Model) renderHeader() string {
	tabs := []string{"1:Search", "2:Results", "3:Downloads", "4:History", "5:Playlist"}
	var renderedTabs []string
	
	for i, t := range tabs {
		style := m.styles.InactiveTab
		if int(m.viewMode) == i {
			style = m.styles.ActiveTab
		}
		renderedTabs = append(renderedTabs, style.Render(t))
	}
	
	return m.styles.Header.Render(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)) + "
"
}

func (m Model) renderSearchView() string {
	var s strings.Builder
	s.WriteString(m.styles.ActiveTitle.Render("── SEARCH TYPE ──") + "

")
	
	choices := []string{"Search Videos", "Search Channels"}
	for i, c := range choices {
		prefix := "  "
		style := m.styles.Subtitle
		if m.searchCursor == i {
			prefix = m.styles.Cursor.Render("> ")
			style = m.styles.ActiveTitle
		}
		s.WriteString(fmt.Sprintf("%s%s
", prefix, style.Render(c)))
	}
	
	s.WriteString("
")
	if m.isSearching {
		s.WriteString(fmt.Sprintf("Search Query: %s_
", m.query))
	} else {
		s.WriteString(m.styles.Meta.Render("Press enter to start typing query for selected type.
"))
	}
	return s.String()
}

func (m Model) renderResultsView(maxHeight int) string {
	if m.isLoadingResults && len(m.results) == 0 {
		return m.styles.MetaHighlight.PaddingLeft(3).Render("
Searching YouTube... Please wait...")
	}

	var s strings.Builder
	itemHeight := 3
	overhead := 2
	maxVisible := (maxHeight - overhead) / itemHeight
	if maxVisible < 1 {
		maxVisible = 1
	}
	
	start, end := m.getVisibleRange(len(m.results), maxVisible)

	if len(m.results) > 0 {
		pageInfo := m.styles.Meta.Render(fmt.Sprintf("Page %d | Results %d-%d of %d", m.page, start+1, end, len(m.results)))
		s.WriteString("   " + pageInfo + "

")
	}

	for i := start; i < end; i++ {
		res := m.results[i]
		
		titleStyle := m.styles.Title
		cursor := "  "
		if m.cursor == i {
			titleStyle = m.styles.ActiveTitle
			cursor = m.styles.Cursor.Render("> ")
		}

		infoStr := res.Duration
		if res.Type == search.ResultTypeChannel { 
			infoStr = res.VideoCount
			if infoStr == "" { infoStr = "Channel" }
		}
		
		title := truncate(res.Title, m.width-12)

		metaParts := []string{truncate(res.ChannelName, 30)}
		if res.Views != "" { metaParts = append(metaParts, res.Views) }
		if res.UploadDate != "" { metaParts = append(metaParts, res.UploadDate) }
		
		metaLine := strings.Join(metaParts, " • ")
		if res.IsDownloaded { metaLine += " 📥" }
		metaLine = truncate(metaLine, m.width-6)

		s.WriteString(fmt.Sprintf("%s%s %s
   %s

", 
			cursor,
			titleStyle.Render(title), 
			m.styles.MetaHighlight.Render("["+infoStr+"]"),
			m.styles.Meta.Render(metaLine)))
	}

	if len(m.results) == 0 && !m.isLoadingResults {
		return m.styles.Meta.PaddingLeft(3).Render("
No results found. Start a search in view 1.")
	}
	
	return s.String()
}

func (m Model) renderProgressBar(percent float64, width int) string {
	if width < 10 { width = 10 }
	filledLen := int(float64(width-2) * percent / 100.0)
	if filledLen < 0 { filledLen = 0 }
	if filledLen > width-2 { filledLen = width-2 }
	
	filled := strings.Repeat("█", filledLen)
	empty := strings.Repeat("░", width-2-filledLen)
	
	return fmt.Sprintf("[%s%s] %.1f%%", m.styles.ActiveTitle.Render(filled), m.styles.Meta.Render(empty), percent)
}

func (m Model) renderDownloadsView() string {
	var s strings.Builder
	
	statusText := m.styles.WorkerActive.Render("Worker: Active")
	if m.isWorkerPaused { 
		statusText = m.styles.WorkerPaused.Render("Worker: Paused") 
	}
	s.WriteString(statusText + "

")

	for i, p := range m.downloads {
		cursor := "  "
		titleStyle := m.styles.Title
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("> ")
			titleStyle = m.styles.ActiveTitle
		}

		if p.IsChannel {
			agg := p.Percentage
			if p.PlaylistTotal > 1 {
				agg = ((float64(p.PlaylistIndex)-1)*100.0 + p.Percentage) / float64(p.PlaylistTotal)
			}
			
			header := fmt.Sprintf("%s[CHN] %s", cursor, p.Title)
			if p.PlaylistTotal > 0 { header += fmt.Sprintf(" (%d videos)", p.PlaylistTotal) }
			s.WriteString(titleStyle.Render(header) + "
")
			
			barWidth := 40
			if m.width > 60 { barWidth = m.width - 20 }
			if barWidth > 80 { barWidth = 80 }
			
			bar := m.renderProgressBar(agg, barWidth)
			s.WriteString(fmt.Sprintf("  %s  %s | %s
", bar, p.Speed, p.ETA))
			
			currentTitle := p.CurrentVideoTitle
			if currentTitle == "" { currentTitle = "Discovering..." }
			
			idxStr := "?"; if p.PlaylistIndex > 0 { idxStr = fmt.Sprintf("%d", p.PlaylistIndex) }
			totStr := "?"; if p.PlaylistTotal > 0 { totStr = fmt.Sprintf("%d", p.PlaylistTotal) }

			videoInfo := fmt.Sprintf("⏳ [%s/%s] %s (%.1f%%)", idxStr, totStr, currentTitle, p.Percentage)
			s.WriteString(m.styles.MetaHighlight.PaddingLeft(4).Render(videoInfo) + "

")
		} else {
			status := "queued"
			if p.Percentage > 0 { status = fmt.Sprintf("%.1f%%", p.Percentage) }
			if m.isWorkerPaused { status = "[PAUSED] " + status }

			line := fmt.Sprintf("%s[VID] %s | %s | %s | %s", cursor, status, p.Speed, p.ETA, p.Title)
			s.WriteString(titleStyle.Render(line) + "
")
		}
	}
	
	if len(m.downloads) == 0 {
		s.WriteString(m.styles.Meta.Render("No active downloads."))
	}
	return s.String()
}

func (m Model) renderHistoryView() string {
	var s strings.Builder
	for i, h := range m.history {
		cursor := "  "
		style := m.styles.Title
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("> ")
			style = m.styles.ActiveTitle
		}
		s.WriteString(fmt.Sprintf("%s%s %s
", cursor, style.Render(h.Title), m.styles.Meta.Render("("+h.UpdatedAt.Format("2006-01-02")+")")))
	}
	if len(m.history) == 0 {
		s.WriteString(m.styles.Meta.Render("No watch history."))
	}
	return s.String()
}

func (m Model) renderPlaylistView() string {
	var s strings.Builder
	for i, p := range m.playlist {
		cursor := "  "
		style := m.styles.Title
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("> ")
			style = m.styles.ActiveTitle
		}
		s.WriteString(fmt.Sprintf("%s%s %s
", cursor, style.Render(p.Title), m.styles.MetaHighlight.Render("["+p.Duration+"]")))
	}
	if len(m.playlist) == 0 {
		s.WriteString(m.styles.Meta.Render("No items in playlist."))
	}
	return s.String()
}

func (m Model) renderDownloadQualityView() string {
	var s strings.Builder
	s.WriteString(m.styles.ActiveTitle.Render("── SELECT DOWNLOAD QUALITY ──") + "

")
	for i, q := range m.qualities {
		prefix := "  "
		style := m.styles.Subtitle
		if m.downloadCursor == i {
			prefix = m.styles.Cursor.Render("> ")
			style = m.styles.ActiveTitle
		}
		s.WriteString(fmt.Sprintf("%s%s
", prefix, style.Render(q)))
	}
	return s.String()
}

func (m Model) renderLegend() string {
	var keys string
	switch m.viewMode {
	case SearchView: keys = "up/down: selection | enter: start typing"
	case ResultsView: keys = "d: download | a: add playlist | n/p: pages | enter: play"
	case DownloadsView: keys = "p: pause worker | r: resume worker | x: cancel"
	case HistoryView: keys = "enter: play"
	case PlaylistView: keys = "enter: play | x: remove"
	case DownloadQualityView: keys = "enter: confirm | esc: cancel"
	}

	status := m.styles.Status.Render("Status: " + m.status)
	help := m.styles.Meta.Render("1-5: views | q: quit | space: pause | " + keys)
	
	return m.styles.Legend.Width(m.width).Render(status + "
" + help)
}

func truncate(s string, max int) string {
	if max < 3 { return s }
	if len(s) > max { return s[:max-3] + "..." }
	return s
}

func (m Model) getVisibleRange(listLen, maxVisible int) (int, int) {
	if listLen <= maxVisible { return 0, listLen }
	start := m.cursor - (maxVisible / 2); if start < 0 { start = 0 }
	end := start + maxVisible
	if end > listLen { end = listLen; start = end - maxVisible }
	return start, end
}
