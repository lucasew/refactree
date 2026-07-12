// Package pprof writes Go runtime profiles to a directory when enabled.
package pprof

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"
)

// Profiler writes cpu/heap/memory/goroutine/allocs profiles under Dir when started.
// Zero value is inert: Start with empty Dir is a no-op, Stop is always safe.
//
// CPU samples accumulate in cpu.pprof and are only complete after Stop.
// Snapshot profiles (heap, memory, goroutine, allocs) are written at Start, on
// SIGUSR1, and again at Stop so long-running commands (e.g. serve) still leave
// usable files if interrupted.
//
// memory.pprof is a post-GC heap snapshot (in-use memory); heap.pprof is the
// live heap without forcing GC first; allocs.pprof is cumulative allocations.
type Profiler struct {
	// Dir is the output directory. Empty means profiling is disabled.
	Dir string

	cpuFile  *os.File
	active   bool
	stopOnce sync.Once
	sigStop  func()
}

// Start begins a CPU profile, writes snapshot profiles, and listens for SIGUSR1
// to refresh snapshots. When Dir is empty, Start is a no-op and returns nil.
func (p *Profiler) Start() error {
	if p == nil || p.Dir == "" {
		return nil
	}
	if p.active {
		return fmt.Errorf("pprof: profiler already started")
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return fmt.Errorf("pprof: mkdir %s: %w", p.Dir, err)
	}

	f, err := os.Create(filepath.Join(p.Dir, "cpu.pprof"))
	if err != nil {
		return fmt.Errorf("pprof: create cpu profile: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return fmt.Errorf("pprof: start cpu profile: %w", err)
	}
	p.cpuFile = f
	p.active = true
	p.writeSnapshots()
	p.watchSignals()
	return nil
}

// Stop ends the CPU profile (if running) and writes final snapshot profiles.
// Safe to call multiple times; no-op when profiling was never started.
func (p *Profiler) Stop() {
	if p == nil {
		return
	}
	p.stopOnce.Do(func() {
		if p.sigStop != nil {
			p.sigStop()
			p.sigStop = nil
		}
		if !p.active {
			return
		}
		p.active = false

		if p.cpuFile != nil {
			pprof.StopCPUProfile()
			_ = p.cpuFile.Close()
			p.cpuFile = nil
		}

		if p.Dir != "" {
			p.writeSnapshots()
		}
	})
}

func (p *Profiler) writeSnapshots() {
	dir := p.Dir
	if dir == "" {
		return
	}
	writeProfile(dir, "heap", func(w io.Writer) error {
		return pprof.WriteHeapProfile(w)
	})
	// Post-GC snapshot: better view of retained (in-use) memory.
	writeProfile(dir, "memory", func(w io.Writer) error {
		runtime.GC()
		return pprof.WriteHeapProfile(w)
	})
	writeProfile(dir, "goroutine", func(w io.Writer) error {
		return pprof.Lookup("goroutine").WriteTo(w, 0)
	})
	writeProfile(dir, "allocs", func(w io.Writer) error {
		return pprof.Lookup("allocs").WriteTo(w, 0)
	})
}

// watchSignals refreshes snapshot profiles on SIGUSR1 without stopping the run.
func (p *Profiler) watchSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	done := make(chan struct{})
	p.sigStop = func() {
		signal.Stop(ch)
		close(done)
	}
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ch:
				if p.active {
					p.writeSnapshots()
				}
			}
		}
	}()
}

func writeProfile(dir, name string, write func(io.Writer) error) {
	path := filepath.Join(dir, name+".pprof")
	f, err := os.Create(path)
	if err != nil {
		slog.Error("pprof: write failed", "path", path, "err", err)
		return
	}
	defer func() { _ = f.Close() }()
	if err := write(f); err != nil {
		slog.Error("pprof: write failed", "path", path, "err", err)
	}
}
