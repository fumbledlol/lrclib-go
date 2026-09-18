# lrclib-go

[![Go Reference](https://pkg.go.dev/badge/github.com/fumbledlol/lrclib-go.svg)](https://pkg.go.dev/github.com/fumbledlol/lrclib-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Go client for the [LRCLIB](https://lrclib.net/) lyrics API and parser for synchronized LRC text files.

## Features

- Fast LRC parser that skips metadata tags and extracts timed lines without regular expressions
- LRCLIB REST client to search tracks, look up by ID, and fetch synchronized or plain lyrics
- Built-in request pacing and automatic backoff on HTTP 429 using `Retry-After`
- Standard library only, no external dependencies

## Installation

```bash
go get github.com/fumbledlol/lrclib-go
```

## Usage

### Parsing synchronized LRC

```go
package main

import (
	"fmt"

	"github.com/fumbledlol/lrclib-go"
)

func main() {
	rawLRC := `[ti:Song Title]
[00:12.34]First lyric line
[00:15.50]Second lyric line`

	lines, err := lrclib.ParseLRC(rawLRC)
	if err != nil {
		panic(err)
	}

	for _, line := range lines {
		fmt.Printf("[%v] (%.2fs) %s\n", line.Timestamp, line.Seconds, line.Text)
	}
}
```

### Fetching lyrics from LRCLIB

```go
client := lrclib.NewClient()

lyrics, err := client.Get(ctx, "Track Name", "Artist Name", "Album Name", 210.0)
if err != nil {
	// Handle ErrNotFound or request error
}

// Access timed lines
lines, err := lyrics.Lines()
```

## Benchmarks

```
BenchmarkParseLRC-12    4112946    306.9 ns/op    352 B/op    1 allocs/op
```

## License

[MIT](LICENSE)