package tokensource

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		line   string
		want   uint64
		wantOK bool
	}{
		{"token 1 - id 123 - timing 0.45s", 1, true},
		{"decode: token 412 - 0.31s", 412, true},
		{"n_cur = 7", 7, true},
		{"token=99", 99, true},
		{"print_token 55", 55, true},
		{"Token 1000 generated", 1000, true},
		{"some unrelated llama.cpp log line", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseLine(c.line)
		if ok != c.wantOK || got != c.want {
			t.Errorf("ParseLine(%q) = (%d, %v), want (%d, %v)", c.line, got, ok, c.want, c.wantOK)
		}
	}
}

func TestTailerTracksTokens(t *testing.T) {
	// Use a pipe so the reader goroutine gets a real streaming reader.
	r, w := io.Pipe()
	go func() {
		// Write a few token-bearing lines interleaved with noise.
		lines := []string{
			"llama.cpp starting up\n",
			"token 0 - timing 0.10s\n",
			"some noise\n",
			"token 5 - timing 0.42s\n",
			"token 5 - timing 0.43s\n", // same index, must not regress
			"token 17 - timing 0.51s\n",
		}
		for _, l := range lines {
			io.WriteString(w, l)
		}
		w.Close()
	}()

	tl := NewFromReader(r)
	tl.Wait()

	if got := tl.CurrentTokens(); got != 17 {
		t.Errorf("CurrentTokens = %d, want 17", got)
	}
}

func TestTailerEmptyInput(t *testing.T) {
	tl := NewFromReader(strings.NewReader(""))
	tl.Wait()
	if got := tl.CurrentTokens(); got != 0 {
		t.Errorf("CurrentTokens on empty input = %d, want 0", got)
	}
}

func TestTailerMonotonicMax(t *testing.T) {
	// A later line with a SMALLER token index must not regress the count.
	r, w := io.Pipe()
	go func() {
		io.WriteString(w, "token 100\n")
		io.WriteString(w, "token 3\n") // out-of-order / replayed log
		io.WriteString(w, "token 105\n")
		w.Close()
	}()
	tl := NewFromReader(r)
	tl.Wait()
	if got := tl.CurrentTokens(); got != 105 {
		t.Errorf("CurrentTokens = %d, want 105 (monotonic max)", got)
	}
}

func TestCloseUnblocksWait(t *testing.T) {
	// A producer holding the write end open (the README fifo setup):
	// Wait must block, and Close must unblock it — v0.1.0 had no Close and
	// attach hung after Ctrl-C in exactly this shape.
	r, w := io.Pipe()
	defer w.Close()
	tl := NewFromReader(r)

	waited := make(chan struct{})
	go func() { tl.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("Wait returned while the producer still holds the pipe open")
	case <-time.After(50 * time.Millisecond):
	}

	if err := tl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after Close")
	}
}

func TestCloseAfterEOF(t *testing.T) {
	// Regular files hit EOF immediately; Close must still work (and the
	// tokens read before EOF must survive).
	tl := NewFromReader(strings.NewReader("token 7\n"))
	tl.Wait()
	if err := tl.Close(); err != nil {
		t.Fatalf("Close after EOF: %v", err)
	}
	if got := tl.CurrentTokens(); got != 7 {
		t.Errorf("CurrentTokens after Close = %d, want 7", got)
	}
}
