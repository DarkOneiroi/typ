// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/darkone/typ/internal/config"
	"github.com/darkone/typ/internal/ipc"
	"github.com/darkone/typ/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	daemonMode = flag.Bool("daemon", false, "Run in daemon mode")
	stopDaemon = flag.Bool("stop", false, "Stop the running daemon")
	socketPath = flag.String("socket", "", "Custom socket path")
	configPath = flag.String("config", "", "Custom config path")
)

func getConfigPath() string {
	if *configPath != "" {
		return *configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "typ", "config.yaml")
}

func getSocketPath() string {
	if *socketPath != "" {
		return *socketPath
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/tmp"
	}
	return filepath.Join(runtimeDir, "typ", "typ.sock")
}

func main() {
	flag.Parse()

	sp := getSocketPath()
	cp := getConfigPath()
	cfg, err := config.LoadConfig(cp)
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	if *stopDaemon {
		client := ipc.NewClient(sp)
		_, err := client.SendRequest(ipc.Request{Type: ipc.ReqStop})
		if err != nil {
			fmt.Printf("Could not stop daemon via IPC: %v. Killing process...
", err)
			exec.Command("pkill", "-f", "typ --daemon").Run()
		} else {
			fmt.Println("Stopped TYP daemon.")
		}
		return
	}

	if *daemonMode {
		d, err := ui.NewDaemon(sp, cfg)
		if err != nil {
			log.Fatalf("Failed to create daemon: %v", err)
		}
		fmt.Printf("Starting TYP Daemon on %s...
", sp)
		if err := d.Start(); err != nil {
			log.Fatalf("Daemon error: %v", err)
		}
		return
	}

	// Client Mode
	client := ipc.NewClient(sp)

	// Try to ping daemon, if not running, start it
	_, err = client.SendRequest(ipc.Request{Type: ipc.ReqStatus})
	if err != nil {
		fmt.Printf("Daemon not running, attempting to start it...
")
		// Start daemon in background
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to get executable path: %v", err)
		}
		
		attr := &os.ProcAttr{
			Dir: ".",
			Env: os.Environ(),
			Files: []*os.File{
				nil, 
				nil, // Hide stdout/stderr to avoid cluttering TUI start
				nil, 
			},
		}
		proc, err := os.StartProcess(exe, []string{exe, "--daemon"}, attr)
		if err != nil {
			log.Fatalf("Failed to start daemon automatically: %v", err)
		}
		
		// Detach
		_ = proc.Release()
		
		// Wait a bit for it to start
		time.Sleep(3 * time.Second)
		
		// Try connecting again to verify it started successfully
		_, err = client.SendRequest(ipc.Request{Type: ipc.ReqStatus})
		if err != nil {
			fmt.Printf("Error: Daemon started but is not responding. Check if dependencies (mpv, yt-dlp) are installed.
")
			os.Exit(1)
		}
	}

	p := tea.NewProgram(ui.NewModel(client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
