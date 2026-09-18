package lrclib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL   = "https://lrclib.net/api"
	DefaultUserAgent = "lrclib-go/1.0 (https://github.com/fumbledlol/lrclib-go)"
	maxResponseBytes = 256 * 1024 // 256 KB
)

var (
	// ErrNotFound is returned when LRCLIB returns 404.
	ErrNotFound = errors.New("lrclib: lyrics not found")
	// ErrRateLimited is returned when LRCLIB rate limits the request and backoff could not recover.
	ErrRateLimited = errors.New("lrclib: rate limited")
)

// Lyrics represents the response payload from LRCLIB.
type Lyrics struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name,omitempty"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName,omitempty"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics,omitempty"`
	SyncedLyrics string  `json:"syncedLyrics,omitempty"`
}

// Lines parses the SyncedLyrics into timed lines.
func (l *Lyrics) Lines() ([]Line, error) {
	if l.SyncedLyrics == "" {
		return nil, nil
	}
	return ParseLRC(l.SyncedLyrics)
}

// SearchParams contains query parameters for searching lyrics.
type SearchParams struct {
	Query      string  `json:"q,omitempty"`
	TrackName  string  `json:"track_name,omitempty"`
	ArtistName string  `json:"artist_name,omitempty"`
	AlbumName  string  `json:"album_name,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
}

// Client interacts with the LRCLIB REST API.
type Client struct {
	baseURL     string
	userAgent   string
	httpClient  *http.Client
	minInterval time.Duration
	mu          sync.Mutex
	lastReqEnd  time.Time
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default API base URL.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) {
		c.userAgent = strings.TrimSpace(ua)
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithMinInterval configures the rate-limiting spacing between requests.
func WithMinInterval(interval time.Duration) ClientOption {
	return func(c *Client) {
		c.minInterval = interval
	}
}

// NewClient creates a new LRCLIB client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL:     DefaultBaseURL,
		userAgent:   DefaultUserAgent,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		minInterval: 250 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) waitRateLimit(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(c.lastReqEnd)
	if elapsed < c.minInterval && !c.lastReqEnd.IsZero() {
		waitDur := c.minInterval - elapsed
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
		}
	}
	c.lastReqEnd = time.Now()
	return nil
}

func (c *Client) doRequest(ctx context.Context, reqURL string) ([]byte, error) {
	var resp *http.Response

	for attempt := range 2 {
		if err := c.waitRateLimit(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Lrclib-Client", c.userAgent)
		req.Header.Set("Accept", "application/json")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			retryAfter := 1 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if s, err := strconv.Atoi(ra); err == nil && s > 0 {
					retryAfter = time.Duration(s) * time.Second
				}
			}
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryAfter):
				continue
			}
		}
		break
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lrclib: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
}

// Get fetches lyrics by exact track and artist metadata.
func (c *Client) Get(ctx context.Context, trackName, artistName, albumName string, duration float64) (*Lyrics, error) {
	q := url.Values{}
	q.Set("track_name", trackName)
	q.Set("artist_name", artistName)
	if albumName != "" {
		q.Set("album_name", albumName)
	}
	if duration > 0 {
		q.Set("duration", strconv.FormatFloat(duration, 'f', 2, 64))
	}

	body, err := c.doRequest(ctx, c.baseURL+"/get?"+q.Encode())
	if err != nil {
		return nil, err
	}

	var l Lyrics
	if err := json.Unmarshal(body, &l); err != nil {
		return nil, fmt.Errorf("lrclib: decode json: %w", err)
	}
	return &l, nil
}

// GetByID fetches lyrics by LRCLIB database ID.
func (c *Client) GetByID(ctx context.Context, id int64) (*Lyrics, error) {
	body, err := c.doRequest(ctx, fmt.Sprintf("%s/get/%d", c.baseURL, id))
	if err != nil {
		return nil, err
	}

	var l Lyrics
	if err := json.Unmarshal(body, &l); err != nil {
		return nil, fmt.Errorf("lrclib: decode json: %w", err)
	}
	return &l, nil
}

// Search searches for lyrics matching query parameters.
func (c *Client) Search(ctx context.Context, params SearchParams) ([]Lyrics, error) {
	q := url.Values{}
	if params.Query != "" {
		q.Set("q", params.Query)
	}
	if params.TrackName != "" {
		q.Set("track_name", params.TrackName)
	}
	if params.ArtistName != "" {
		q.Set("artist_name", params.ArtistName)
	}
	if params.AlbumName != "" {
		q.Set("album_name", params.AlbumName)
	}
	if params.Duration > 0 {
		q.Set("duration", strconv.FormatFloat(params.Duration, 'f', 2, 64))
	}

	body, err := c.doRequest(ctx, c.baseURL+"/search?"+q.Encode())
	if err != nil {
		return nil, err
	}

	var list []Lyrics
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("lrclib: decode json: %w", err)
	}
	return list, nil
}
