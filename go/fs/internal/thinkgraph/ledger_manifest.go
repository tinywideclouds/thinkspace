package thinkgraph

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const maxLedgerLineBytes = 10 * 1024 * 1024

type LedgerManifest struct {
	ActiveShard string `json:"active_shard"`
	EventCount  int    `json:"event_count"`
	TotalEvents int    `json:"total_events"`
}

func LoadOrRebuildManifest(ledgerDirectory string, subGraphName string) (*LedgerManifest, error) {
	dirName := ResolveLogDirName(subGraphName)
	branchDir := filepath.Join(ledgerDirectory, "logs", dirName)
	if err := os.MkdirAll(branchDir, 0755); err != nil {
		return nil, fmt.Errorf("creating branch log directory %s: %w", branchDir, err)
	}

	manifestPath := filepath.Join(branchDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		var m LedgerManifest
		if unmarshalErr := json.Unmarshal(data, &m); unmarshalErr == nil && m.ActiveShard != "" {
			return &m, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading manifest %s: %w", manifestPath, err)
	}

	return RebuildManifest(ledgerDirectory, dirName)
}

func RebuildManifest(ledgerDirectory string, subGraphName string) (*LedgerManifest, error) {
	dirName := ResolveLogDirName(subGraphName)
	branchDir := filepath.Join(ledgerDirectory, "logs", dirName)
	entries, err := os.ReadDir(branchDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &LedgerManifest{ActiveShard: "00000.jsonl", EventCount: 0, TotalEvents: 0}, nil
		}
		return nil, fmt.Errorf("reading logs branch directory: %w", err)
	}

	var shards []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			shards = append(shards, e.Name())
		}
	}

	if len(shards) == 0 {
		return &LedgerManifest{ActiveShard: "00000.jsonl", EventCount: 0, TotalEvents: 0}, nil
	}

	sort.Strings(shards)
	activeShard := shards[len(shards)-1]

	totalCount := 0
	eventCount := 0

	for _, shard := range shards {
		shardPath := filepath.Join(branchDir, shard)
		file, openErr := os.Open(shardPath)
		if openErr != nil {
			return nil, fmt.Errorf("opening shard %s: %w", shardPath, openErr)
		}

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), maxLedgerLineBytes)
		shardCount := 0
		for scanner.Scan() {
			if len(scanner.Bytes()) > 0 {
				shardCount++
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("scanning shard %s: %w", shardPath, scanErr)
		}

		totalCount += shardCount
		if shard == activeShard {
			eventCount = shardCount
		}
	}

	m := &LedgerManifest{ActiveShard: activeShard, EventCount: eventCount, TotalEvents: totalCount}
	if err := m.Save(ledgerDirectory, dirName); err != nil {
		return nil, fmt.Errorf("saving rebuilt manifest: %w", err)
	}
	return m, nil
}

func (m *LedgerManifest) Save(ledgerDirectory string, subGraphName string) error {
	dirName := ResolveLogDirName(subGraphName)
	branchDir := filepath.Join(ledgerDirectory, "logs", dirName)
	if err := os.MkdirAll(branchDir, 0755); err != nil {
		return fmt.Errorf("creating branch log directory %s: %w", branchDir, err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	tmpFile, err := os.CreateTemp(branchDir, ".manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp manifest in %s: %w", branchDir, err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp manifest: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp manifest: %w", err)
	}

	destPath := filepath.Join(branchDir, "manifest.json")
	var renameErr error
	maxRetries := 1
	if runtime.GOOS == "windows" {
		maxRetries = 10
	}
	for attempt := 0; attempt < maxRetries; attempt++ {
		renameErr = os.Rename(tmpName, destPath)
		if renameErr == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}

	_ = os.Remove(tmpName)
	return fmt.Errorf("renaming temp manifest: %w", renameErr)
}
