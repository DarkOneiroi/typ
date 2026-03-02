package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

// MpvPlayer implements the Player interface using an external mpv process.
type MpvPlayer struct {
	cmd        *exec.Cmd
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
	status     Status
	stopChan   chan struct{}
}

// NewMpvPlayer creates a new MpvPlayer instance.
func NewMpvPlayer(socketPath string) (*MpvPlayer, error) {
	p := &MpvPlayer{
		socketPath: socketPath,
		stopChan:   make(chan struct{}),
		status: Status{
			Volume: 100,
			State:  StateIdle,
		},
	}

	if err := p.startMpv(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *MpvPlayer) startMpv() error {
	// Ensure mpv is in PATH
	if _, err := exec.LookPath("mpv"); err != nil {
		return fmt.Errorf("mpv not found in PATH: please install it (e.g., sudo pacman -S mpv)")
	}

	args := []string{
		"--idle",
		"--input-ipc-server=" + p.socketPath,
		"--ytdl-format=bestvideo[height<=1080]+bestaudio/best[height<=1080]/best",
		"--cache=yes",
		"--demuxer-max-bytes=500M",
		"--demuxer-max-back-bytes=100M",
		"--ytdl-raw-options=ignore-config=,sub-format=en,write-sub=",
		"--vd-lavc-threads=4",
	}

	p.cmd = exec.Command("mpv", args...)
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mpv: %w", err)
	}

	// Wait for socket to be created
	var conn net.Conn
	var err error
	for i := 0; i < 20; i++ {
		conn, err = net.Dial("unix", p.socketPath)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to mpv socket: %w", err)
	}

	p.conn = conn
	go p.listenEvents()

	return nil
}

func (p *MpvPlayer) listenEvents() {
	// Start a ticker to poll for position
	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-p.stopChan:
				return
			case <-ticker.C:
				p.sendCommand("get_property", "time-pos")
			}
		}
	}()

	scanner := bufio.NewScanner(p.conn)
	for scanner.Scan() {
		var event map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		// Handle data responses (from get_property)
		if val, ok := event["data"]; ok {
			if pos, ok := val.(float64); ok {
				p.mu.Lock()
				p.status.Position = time.Duration(pos * float64(time.Second))
				p.mu.Unlock()
			}
		}

		// Handle events (like end-of-file, property-change)
		if eventName, ok := event["event"].(string); ok {
			p.handleEvent(eventName, event)
		}
	}
}

func (p *MpvPlayer) handleEvent(name string, data map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch name {
	case "start-file":
		p.status.State = StatePlaying
	case "end-file":
		p.status.State = StateIdle
		p.status.CurrentTrack = nil
	case "pause":
		p.status.State = StatePaused
	case "unpause":
		p.status.State = StatePlaying
	}
}

func (p *MpvPlayer) sendCommand(args ...interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmd := map[string]interface{}{
		"command": args,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	_, err = p.conn.Write(append(data, '\n'))
	return err
}

// Interface Implementation

func (p *MpvPlayer) Load(track Track, autoPlay bool) error {
	p.mu.Lock()
	p.status.CurrentTrack = &track
	p.mu.Unlock()

	mode := "replace"
	if !autoPlay {
		// mpv doesn't have a direct "load but don't play" that works reliably with idle
		// we usually load and then pause if needed
	}

	err := p.sendCommand("loadfile", track.URL, mode)
	if err != nil {
		return err
	}

	if !autoPlay {
		return p.Pause()
	}
	return nil
}

func (p *MpvPlayer) Play() error {
	return p.sendCommand("set_property", "pause", false)
}

func (p *MpvPlayer) Pause() error {
	return p.sendCommand("set_property", "pause", true)
}

func (p *MpvPlayer) TogglePause() error {
	// We can't easily get the property back synchronously here without more complex IPC
	// For now, we'll assume the state we have is correct or use 'cycle pause'
	return p.sendCommand("cycle", "pause")
}

func (p *MpvPlayer) Stop() error {
	return p.sendCommand("stop")
}

func (p *MpvPlayer) Seek(position time.Duration) error {
	return p.sendCommand("seek", position.Seconds(), "absolute")
}

func (p *MpvPlayer) SetVolume(volume int) error {
	p.mu.Lock()
	p.status.Volume = volume
	p.mu.Unlock()
	return p.sendCommand("set_property", "volume", volume)
}

func (p *MpvPlayer) GetStatus() (Status, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status, nil
}

func (p *MpvPlayer) Close() error {
	close(p.stopChan)
	if p.conn != nil {
		p.conn.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	os.Remove(p.socketPath)
	return nil
}
