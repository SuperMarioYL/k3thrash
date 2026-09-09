// Package tokensource tails llama.cpp's per-token timing output (stderr or a
// redirected file/pipe) to recover the cumulative decode-token count, which
// the attach command correlates with /proc/<pid>/io.read_bytes to derive the
// per-token NVMe read-rate the thrash engine judges.
//
// The tailer is cross-platform (it just reads lines from an io.Reader / file
// path); how you wire it to a running llama.cpp K3 process's stderr is your
// environment's concern (the happy path is `llama-cli ... 2> /tmp/k3.fifo`
// then `k3thrash attach --pid $(pgrep -f llama) --token-source /tmp/k3.fifo`).
package tokensource

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"sync/atomic"
)

// Tailer reads lines from a token-timing stream and tracks the highest
// decode-token index seen so far. CurrentTokens is safe to call from a
// different goroutine than the reader.
type Tailer struct {
	tokens atomic.Uint64
	done   chan struct{}
	// closer, when non-nil, is the underlying reader owned by the Tailer;
	// Close closes it so a producer-held fifo/pipe can no longer block
	// shutdown.
	closer io.Closer
}

var tokenPatterns = []*regexp.Regexp{
	// llama.cpp "prompt eval" / "decode" progress: "token N - ..." or
	// "decode: token 123" or "n_cur = 123".
	regexp.MustCompile(`(?i)\btoken\s+(\d+)`),
	regexp.MustCompile(`(?i)\bn_cur\s*=\s*(\d+)`),
	// "print_token 123" / "token=123"
	regexp.MustCompile(`(?i)token[=: ]+(\d+)`),
}

// ParseLine extracts the cumulative token index from one stderr line. It
// returns (n, true) on a match where n is >= the prior count (the caller maxes
// it). ok=false means the line had no token signal.
func ParseLine(line string) (uint64, bool) {
	for _, re := range tokenPatterns {
		m := re.FindStringSubmatch(line)
		if len(m) >= 2 {
			v, err := strconv.ParseUint(m[1], 10, 64)
			if err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

// NewFromReader constructs a Tailer that consumes lines from r in a background
// goroutine until the reader hits EOF or errors. Useful for tests with a
// pipe; New opens a file path instead.
func NewFromReader(r io.Reader) *Tailer {
	t := &Tailer{done: make(chan struct{})}
	if c, ok := r.(io.Closer); ok {
		t.closer = c
	}
	go t.run(r)
	return t
}

// New opens path (or stdin if path == "-") and tails it. Returns an error if
// the path cannot be opened; the tailer runs until the file is closed.
func New(path string) (*Tailer, error) {
	var r io.Reader
	if path == "-" || path == "" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("k3thrash: open token-source %q: %w", path, err)
		}
		r = f
	}
	return NewFromReader(r), nil
}

func (t *Tailer) run(r io.Reader) {
	defer close(t.done)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if n, ok := ParseLine(line); ok {
			for {
				cur := t.tokens.Load()
				if n <= cur {
					break
				}
				if t.tokens.CompareAndSwap(cur, n) {
					break
				}
			}
		}
	}
}

// CurrentTokens returns the highest token index seen so far.
func (t *Tailer) CurrentTokens() uint64 {
	return t.tokens.Load()
}

// Wait blocks until the reader goroutine has finished (EOF or error).
func (t *Tailer) Wait() {
	<-t.done
}

// Close releases the token source and waits for the reader goroutine to
// finish. Closing the underlying reader (file, fifo, stdin, pipe) unblocks a
// pending Read, so a producer that keeps its write end open — the documented
// `llama-cli ... 2> /tmp/k3.fifo` setup — can no longer hang attach shutdown
// after Ctrl-C. On readers that are not io.Closers it degrades to Wait.
func (t *Tailer) Close() error {
	var err error
	if c := t.closer; c != nil {
		err = c.Close()
	}
	t.Wait()
	return err
}
