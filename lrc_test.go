package lrclib

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const sampleLRC = `[ti:Song Title]
[ar:Artist Name]
[al:Album Name]
[00:12.34]First line of the song
[00:15.500]Second line with millisecond precision
[01:05.10]Third line after one minute
[02:30.99]Outro line`

func TestParseLRC(t *testing.T) {
	lines, err := ParseLRC(sampleLRC)
	if err != nil {
		t.Fatalf("ParseLRC failed: %v", err)
	}

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	if lines[0].Text != "First line of the song" {
		t.Errorf("unexpected text: %q", lines[0].Text)
	}
	if lines[0].Seconds != 12.34 {
		t.Errorf("unexpected seconds: %f", lines[0].Seconds)
	}
	if lines[0].Timestamp != 12340*time.Millisecond {
		t.Errorf("unexpected timestamp: %v", lines[0].Timestamp)
	}

	if lines[1].Seconds != 15.5 {
		t.Errorf("unexpected seconds: %f", lines[1].Seconds)
	}

	if lines[2].Seconds != 65.1 {
		t.Errorf("unexpected seconds: %f", lines[2].Seconds)
	}
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": 12345,
			"trackName": "Sample Track",
			"artistName": "Sample Artist",
			"duration": 210.5,
			"instrumental": false,
			"plainLyrics": "Hello world",
			"syncedLyrics": "[00:10.00]Hello world"
		}`))
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithMinInterval(0),
	)

	l, err := client.Get(context.Background(), "Sample Track", "Sample Artist", "", 0)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if l.ID != 12345 {
		t.Errorf("unexpected ID: %d", l.ID)
	}

	lines, err := l.Lines()
	if err != nil {
		t.Fatalf("Lines() failed: %v", err)
	}
	if len(lines) != 1 || lines[0].Text != "Hello world" {
		t.Errorf("unexpected parsed lines: %+v", lines)
	}
}

func BenchmarkParseLRC(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = ParseLRC(sampleLRC)
	}
}
