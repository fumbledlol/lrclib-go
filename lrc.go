package lrclib

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidLRC indicates the input is not valid LRC formatted text.
	ErrInvalidLRC = errors.New("lrclib: invalid or empty LRC data")
)

// Line represents a single synchronized lyric line.
type Line struct {
	Timestamp time.Duration `json:"timestamp"`
	Seconds   float64       `json:"seconds"`
	TimeStr   string        `json:"time_str"`
	Text      string        `json:"text"`
}

// ParseLRC parses synchronized LRC text into a slice of timed lines.
// It skips
// metadata tags (e.g. [ar:...]) and extracts all valid timestamped lines.
func ParseLRC(syncedLyrics string) ([]Line, error) {
	if len(syncedLyrics) == 0 {
		return nil, ErrInvalidLRC
	}

	// Heuristic line count estimation for slice preallocation
	estimatedLines := min(strings.Count(syncedLyrics, "\n")+1, 1000)
	lines := make([]Line, 0, estimatedLines)

	remaining := syncedLyrics
	for len(remaining) > 0 {
		var line string
		line, remaining, _ = strings.Cut(remaining, "\n")

		line = strings.TrimSpace(line)
		if len(line) < 7 || line[0] != '[' {
			continue
		}

		parsed, ok := parseLineFast(line)
		if ok {
			lines = append(lines, parsed)
		}
	}

	if len(lines) == 0 {
		return nil, ErrInvalidLRC
	}

	return lines, nil
}

// parseLineFast parses a single LRC line like "[01:23.45] lyric text" or "[0:12.345]text".
func parseLineFast(line string) (Line, bool) {
	tag, text, ok := strings.Cut(line, "]")
	if !ok || len(tag) < 6 || len(tag) > 11 || tag[0] != '[' {
		return Line{}, false
	}

	minStr, secFrac, ok := strings.Cut(tag[1:], ":")
	if !ok || len(minStr) < 1 {
		return Line{}, false
	}

	secStr, fracStr, ok := strings.Cut(secFrac, ".")
	if !ok {
		return Line{}, false
	}

	minutes, ok := parseDecimal(minStr)
	if !ok {
		return Line{}, false
	}

	sec, ok := parseDecimal(secStr)
	if !ok {
		return Line{}, false
	}

	frac, ok := parseDecimal(fracStr)
	if !ok {
		return Line{}, false
	}

	// Normalize fraction to milliseconds
	var fracMs int64
	switch len(fracStr) {
	case 1:
		fracMs = frac * 100
	case 2:
		fracMs = frac * 10
	case 3:
		fracMs = frac
	default:
		fracMs = frac
		for len(fracStr) > 3 {
			fracMs /= 10
			fracStr = fracStr[:len(fracStr)-1]
		}
	}

	totalMs := minutes*60*1000 + sec*1000 + fracMs
	dur := time.Duration(totalMs) * time.Millisecond
	seconds := float64(totalMs) / 1000.0
	timeStr := tag[1:]
	text = strings.TrimSpace(text)

	return Line{
		Timestamp: dur,
		Seconds:   seconds,
		TimeStr:   timeStr,
		Text:      text,
	}, true
}

func parseDecimal(s string) (int64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	var n int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	return n, true
}
