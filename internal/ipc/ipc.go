// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package ipc

import (
	"encoding/json"
	"net"
)

// RequestType identifies the type of IPC request.
type RequestType string

const (
	ReqPlay     RequestType = "play"
	ReqPause    RequestType = "pause"
	ReqStop     RequestType = "stop"
	ReqSeek     RequestType = "seek"
	ReqStatus   RequestType = "status"
	ReqSearch   RequestType = "search"
	ReqSearchStatus RequestType = "search_status"
	ReqDownload     RequestType = "download"
	ReqDownloadsStatus RequestType = "downloads_status"
	ReqHistory  RequestType = "history"
	ReqPlaylist RequestType = "playlist"
	ReqAddToPlaylist RequestType = "add_to_playlist"
	ReqRemoveFromPlaylist RequestType = "remove_from_playlist"
	ReqCreatePlaylist RequestType = "create_playlist"
	ReqListPlaylists RequestType = "list_playlists"
	ReqPauseDownload RequestType = "pause_download"
	ReqResumeDownload RequestType = "resume_download"
	ReqCancelDownload RequestType = "cancel_download"
)

// Request represents a message from the TUI to the Daemon.
type Request struct {
	Type    RequestType            `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// Response represents a message from the Daemon to the TUI.
type Response struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// Client provides a high-level API for communicating with the TYP daemon.
type Client struct {
	socketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) SendRequest(req Request) (*Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
