package aof

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"tinyred/resp"
)

type Config struct {
	Dir            string
	Appenddirname  string
	Appendfilename string
	Appendfsync    string
}

type ReplayFunc func(args []string) error

type AOF interface {
	Load(replay ReplayFunc) error
	AppendCmd(cmd string, args []string) error
	Close() error
}

type AOFLocal struct {
	mu          sync.Mutex
	dir         string
	manifest    *Manifest
	filePath    string
	file        *os.File
	fsyncPolicy string
}

// New initializes the AOF directory, ensures the manifest file exists,
// opens the active increment file for reading & appending, and returns an AOFLocal instance.
func New(cfg Config) (*AOFLocal, error) {
	appendDir := filepath.Join(cfg.Dir, cfg.Appenddirname)
	if err := os.MkdirAll(appendDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create append directory: %w", err)
	}

	manifest, err := EnsureManifest(appendDir, cfg.Appendfilename)
	if err != nil {
		return nil, fmt.Errorf("failed to load or create manifest: %w", err)
	}

	aofFilePath := filepath.Join(appendDir, manifest.AofFilename)

	// Open or create the active AOF file in read-write append mode
	file, err := os.OpenFile(aofFilePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open AOF file %s: %w", aofFilePath, err)
	}

	return &AOFLocal{
		dir:         appendDir,
		manifest:    manifest,
		filePath:    aofFilePath,
		file:        file,
		fsyncPolicy: cfg.Appendfsync,
	}, nil
}

// Load replays existing commands from the beginning of the AOF file.
func (a *AOFLocal) Load(replay ReplayFunc) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := a.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("AOF seek start error: %w", err)
	}

	reader := bufio.NewReader(a.file)
	for {
		arr := resp.Array{}
		err := arr.Unmarshal(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("AOF parse error: %w", err)
		}
		if len(arr.Value) == 0 {
			return fmt.Errorf("replay error: empty command array")
		}

		if err := replay(arr.Value); err != nil {
			return fmt.Errorf("replay error: %w", err)
		}
	}

	// Seek to end of file so subsequent appends write to the end
	if _, err := a.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("AOF seek end error: %w", err)
	}

	return nil
}

// AppendCmd formats a Redis command and its arguments into RESP array format and writes to AOF.
func (a *AOFLocal) AppendCmd(cmd string, args []string) error {
	args = append([]string{cmd}, args...)
	arr := resp.Array{}
	for _, arg := range args {
		bs := resp.BulkString{Value: arg}
		arr.Value = append(arr.Value, string(bs.Marshal()))
	}
	return a.Append(arr.Marshal())
}

// Append writes raw bytes to the open AOF file and flushes based on fsyncPolicy.
func (a *AOFLocal) Append(data []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := a.file.Write(data); err != nil {
		return fmt.Errorf("failed to write to AOF file: %w", err)
	}

	if a.fsyncPolicy == "always" {
		if err := a.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync AOF file: %w", err)
		}
	}

	return nil
}

// Close cleanly closes the open AOF file handle.
func (a *AOFLocal) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.file != nil {
		err := a.file.Close()
		a.file = nil
		return err
	}
	return nil
}
