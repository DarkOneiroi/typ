package search

import (
	"context"
)

// ResultType indicates whether the result is a video, playlist, or channel.
type ResultType string

const (
	ResultTypeVideo    ResultType = "video"
	ResultTypePlaylist ResultType = "playlist"
	ResultTypeChannel  ResultType = "channel"
)

// Result represents a single search result.
type Result struct {
	ID           string
	Type         ResultType
	Title        string
	ChannelName  string
	ChannelURL   string
	Duration     string
	VideoCount   string // For channels
	Thumbnail    string
	URL          string
	Views        string
	UploadDate   string
	Description  string
	IsDownloaded bool
	WatchedPos   string // e.g. "1:20 / 10:00"
}

// Searcher defines the contract for finding content.
type Searcher interface {
	// Search performs a query and returns a channel that emits results as they are found.
	Search(ctx context.Context, query string, resultType ResultType, page int) (<-chan Result, error)

	// Suggest returns autocomplete suggestions for a partial query.
	Suggest(ctx context.Context, query string) ([]string, error)
}
