// Package buildinfo identifies the running binary, so that "which build is
// doing this work" is a recorded fact rather than an inference.
//
// # WHY THIS EXISTS
//
// An hour was lost to a `data-universe` process launched the previous evening
// still running on the OLD binary while a freshly built one worked the same
// claim queue. Both claimed rows atomically, neither errored, and the only
// symptom was a new column populated for 64.5% of bars — which reads as a bug
// in the column rather than as two writers.
//
// Rebuilding a file does not restart a running process, and this repo's
// workers are long-lived daemons with internal timers. So the version is
// stamped on every claimed row and logged at startup, and a queue worked by
// more than one version becomes a one-query observation.
//
// # WHY A CONTENT HASH RATHER THAN A VERSION STRING
//
// This repo has no VCS metadata, so debug.ReadBuildInfo carries no revision.
// A hand-maintained constant would be worse than nothing: it is exactly the
// thing someone forgets to bump on the rebuild that matters, and a stale
// version string would have made the incident above harder to see, not easier.
//
// Hashing the executable cannot be forgotten. Two different builds always
// differ; the same build always agrees.
package buildinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var (
	once    sync.Once
	version string
)

// Version returns a short, stable identifier for the running executable.
//
// Computed once and cached: the binary cannot change underneath a running
// process in any way that matters here, and re-hashing per claim would put
// file I/O on the hot path.
//
// Never returns an empty string. If the executable cannot be read — an
// unusual container layout, a deleted binary — it degrades to a
// start-time-derived value with an explicit `unknown-` prefix, because a
// blank stamp would silently defeat the multi-version check it exists to
// enable.
func Version() string {
	once.Do(func() { version = compute() })
	return version
}

func compute() string {
	path, err := os.Executable()
	if err != nil {
		return fallback("no-executable-path")
	}
	f, err := os.Open(path)
	if err != nil {
		return fallback("unreadable-executable")
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fallback("unhashable-executable")
	}
	// 12 hex characters: enough that a collision between two builds of the
	// same service is not a practical concern, short enough to read in a log
	// line and a table column.
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func fallback(reason string) string {
	return fmt.Sprintf("unknown-%s-%d", reason, time.Now().Unix())
}
