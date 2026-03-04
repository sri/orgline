package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var watchedExtensions = map[string]struct{}{
	".go":   {},
	".html": {},
	".css":  {},
	".js":   {},
	".ts":   {},
	".sql":  {},
	".json": {},
	".md":   {},
}

var watchedFilenames = map[string]struct{}{
	"justfile": {},
}

type fileStamp struct {
	modTime time.Time
	size    int64
}

type pollWatcher struct {
	root string
	last map[string]fileStamp
}

func newPollWatcher(root string) (*pollWatcher, error) {
	snapshot, err := scanFiles(root)
	if err != nil {
		return nil, err
	}

	return &pollWatcher{
		root: root,
		last: snapshot,
	}, nil
}

func (w *pollWatcher) Changed() (bool, string, error) {
	current, err := scanFiles(w.root)
	if err != nil {
		return false, "", err
	}

	for path, now := range current {
		prev, exists := w.last[path]
		if !exists {
			w.last = current
			return true, path, nil
		}
		if now.size != prev.size || !now.modTime.Equal(prev.modTime) {
			w.last = current
			return true, path, nil
		}
	}

	for path := range w.last {
		if _, exists := current[path]; !exists {
			w.last = current
			return true, path, nil
		}
	}

	w.last = current
	return false, "", nil
}

func scanFiles(root string) (map[string]fileStamp, error) {
	stamps := make(map[string]fileStamp)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "bin", "tmp", "node_modules":
				return filepath.SkipDir
			default:
				return nil
			}
		}

		if !d.Type().IsRegular() {
			return nil
		}

		if !shouldWatchFile(name) {
			return nil
		}

		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-shm") || strings.HasSuffix(name, ".db-wal") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		relPath := filepath.ToSlash(path)
		stamps[relPath] = fileStamp{
			modTime: info.ModTime().UTC().Round(time.Millisecond),
			size:    info.Size(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return stamps, nil
}

func shouldWatchFile(name string) bool {
	lowerName := strings.ToLower(name)
	if _, ok := watchedFilenames[lowerName]; ok {
		return true
	}
	ext := filepath.Ext(lowerName)
	_, ok := watchedExtensions[ext]
	return ok
}

type runningProcess struct {
	cmd  *exec.Cmd
	done chan error
}

type serverRunner struct {
	addr   string
	dbPath string
	proc   *runningProcess
}

func (r *serverRunner) Start() error {
	if r.proc != nil {
		return nil
	}
	return r.startLocked()
}

func (r *serverRunner) Restart() error {
	if err := r.Stop(3 * time.Second); err != nil {
		return err
	}
	return r.startLocked()
}

func (r *serverRunner) Stop(timeout time.Duration) error {
	if r.proc == nil {
		return nil
	}

	proc := r.proc
	r.proc = nil
	if proc.cmd.Process == nil {
		return nil
	}

	pid := proc.cmd.Process.Pid
	_ = signalProcessGroup(pid, syscall.SIGINT)
	select {
	case err := <-proc.done:
		if !isExpectedStopError(err) {
			return fmt.Errorf("web process stopped with error: %w", err)
		}
		return nil
	case <-time.After(timeout):
		_ = signalProcessGroup(pid, syscall.SIGTERM)
		select {
		case err := <-proc.done:
			if !isExpectedStopError(err) {
				return fmt.Errorf("web process stopped after SIGTERM with error: %w", err)
			}
			return nil
		case <-time.After(750 * time.Millisecond):
		}

		_ = signalProcessGroup(pid, syscall.SIGKILL)
		err := <-proc.done
		if !isExpectedStopError(err) {
			return fmt.Errorf("web process killed after timeout: %w", err)
		}
		return nil
	}
}

func (r *serverRunner) startLocked() error {
	devBuildID := time.Now().UTC().Format(time.RFC3339Nano)
	cmd := exec.Command(
		"go", "run", "./cmd/web",
		"-addr", r.addr,
		"-db-path", r.dbPath,
		"-dev",
		"-dev-build-id", devBuildID,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Env = append(os.Environ(),
		"ORGLINE_DEV_MODE=1",
		"ORGLINE_DEV_BUILD_ID="+devBuildID,
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start web process: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	r.proc = &runningProcess{
		cmd:  cmd,
		done: done,
	}

	log.Printf("started web process pid=%d build=%s", cmd.Process.Pid, devBuildID)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addrFlag := flag.String("addr", envOrDefault("ORGLINE_ADDR", ":8080"), "Address to pass to cmd/web.")
	dbPathFlag := flag.String("db-path", envOrDefault("ORGLINE_DB_PATH", "orgline.db"), "SQLite path to pass to cmd/web.")
	pollFlag := flag.Duration("poll", 500*time.Millisecond, "File watch polling interval.")
	flag.Parse()

	watcher, err := newPollWatcher(".")
	if err != nil {
		log.Fatal(err)
	}

	runner := &serverRunner{
		addr:   *addrFlag,
		dbPath: *dbPathFlag,
	}
	if err := runner.Start(); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := runner.Stop(3 * time.Second); err != nil {
			log.Printf("stop web process: %v", err)
		}
	}()

	ticker := time.NewTicker(*pollFlag)
	defer ticker.Stop()

	log.Printf("watching files for changes every %s", pollFlag.String())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, changedPath, err := watcher.Changed()
			if err != nil {
				log.Printf("watch error: %v", err)
				continue
			}
			if !changed {
				continue
			}

			log.Printf("change detected: %s", changedPath)
			if err := runner.Restart(); err != nil {
				log.Printf("restart failed: %v", err)
			}
		}
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func signalProcessGroup(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}

	err := syscall.Kill(-pid, sig)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func isExpectedStopError(err error) bool {
	if err == nil {
		return true
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	if status.Signaled() {
		sig := status.Signal()
		return sig == syscall.SIGINT || sig == syscall.SIGTERM || sig == syscall.SIGKILL
	}

	return exitErr.ExitCode() == 130 || exitErr.ExitCode() == 143
}
