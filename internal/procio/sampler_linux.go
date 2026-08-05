//go:build linux

package procio

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// newProcSampler opens /proc/<pid>/io for the given pid. The path is verified
// to exist and be readable so the attach command fails fast with a clear
// error instead of streaming no-data samples.
func newProcSampler(pid int) (Sampler, error) {
	path := fmt.Sprintf("/proc/%d/io", pid)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("k3thrash: cannot stat %s: %w (is pid %d alive?)", path, err, pid)
	}
	return &procSampler{path: path, pid: pid}, nil
}

type procSampler struct {
	path string
	pid  int
}

func (s *procSampler) Read() (IOSample, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return IOSample{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var readBytes uint64
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "read_bytes:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return IOSample{}, fmt.Errorf("k3thrash: parse read_bytes in %q: %w", line, err)
		}
		readBytes = v
		break
	}
	if err := sc.Err(); err != nil {
		return IOSample{}, err
	}
	return IOSample{T: now(), ReadBytes: readBytes}, nil
}

func (*procSampler) Close() error { return nil }
