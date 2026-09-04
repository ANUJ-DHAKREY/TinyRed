package aof

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Manifest struct {
	NodeType    string // "file"
	AofFilename string // "appendonly.aof.1.incr.aof"
	Seq         int    // 1
	FileType    byte   // 'i'
}

// ParseManifest parses the content of a manifest file.
// Format example: "file appendonly.aof.1.incr.aof seq 1 type i\n"
func ParseManifest(content string) (*Manifest, error) {
	fields := strings.Fields(content)
	if len(fields) < 6 {
		return nil, fmt.Errorf("corrupted manifest file")
	}

	seq, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, fmt.Errorf("invalid sequence in manifest: %w", err)
	}

	return &Manifest{
		NodeType:    fields[0],
		AofFilename: fields[1],
		Seq:         seq,
		FileType:    fields[5][0],
	}, nil
}

// FormatManifest produces the manifest file content string.
func FormatManifest(m *Manifest) string {
	return fmt.Sprintf("%s %s seq %d type %c\n", m.NodeType, m.AofFilename, m.Seq, m.FileType)
}

// EnsureManifest checks if the manifest file exists. If not, it creates a default
// manifest pointing to the first increment file (e.g. appendonly.aof.1.incr.aof).
func EnsureManifest(dir, appendFilename string) (*Manifest, error) {
	manifestPath := filepath.Join(dir, appendFilename+".manifest")

	if _, err := os.Stat(manifestPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		defaultAofFilename := fmt.Sprintf("%s.1.incr.aof", appendFilename)
		manifest := &Manifest{
			NodeType:    "file",
			AofFilename: defaultAofFilename,
			Seq:         1,
			FileType:    'i',
		}
		content := []byte(FormatManifest(manifest))
		if err := os.WriteFile(manifestPath, content, 0644); err != nil {
			return nil, err
		}
		return manifest, nil
	}

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	return ParseManifest(string(content))
}
