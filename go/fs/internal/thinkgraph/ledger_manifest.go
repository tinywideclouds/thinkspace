package thinkgraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LedgerManifest struct {
	ActiveShard string `json:"active_shard"`
	EventCount  int    `json:"event_count"`
	TotalEvents int    `json:"total_events"`
}

func LoadOrRebuildManifest(ledgerDirectory string, subGraphName string) (*LedgerManifest, error) {
	if subGraphName == "" {
		subGraphName = "main"
	}
	branchDir := filepath.Join(ledgerDirectory, "logs", subGraphName)
	_ = os.MkdirAll(branchDir, 0755)

	manifestPath := filepath.Join(branchDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		var m LedgerManifest
		if err := json.Unmarshal(data, &m); err == nil {
			return &m, nil
		}
	}
	return RebuildManifest(ledgerDirectory, subGraphName)
}

func RebuildManifest(ledgerDirectory string, subGraphName string) (*LedgerManifest, error) {
	if subGraphName == "" {
		subGraphName = "main"
	}
	branchDir := filepath.Join(ledgerDirectory, "logs", subGraphName)
	entries, err := os.ReadDir(branchDir)
	if err != nil {
		if os.IsNotExist(err) {
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
		file, err := os.Open(filepath.Join(branchDir, shard))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		shardCount := 0
		for scanner.Scan() {
			if len(scanner.Bytes()) > 0 {
				shardCount++
			}
		}
		totalCount += shardCount
		if shard == activeShard {
			eventCount = shardCount
		}
		file.Close()
	}

	m := &LedgerManifest{ActiveShard: activeShard, EventCount: eventCount, TotalEvents: totalCount}
	_ = m.Save(ledgerDirectory, subGraphName)
	return m, nil
}

func (m *LedgerManifest) Save(ledgerDirectory string, subGraphName string) error {
	if subGraphName == "" {
		subGraphName = "main"
	}
	branchDir := filepath.Join(ledgerDirectory, "logs", subGraphName)
	_ = os.MkdirAll(branchDir, 0755)

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(branchDir, "manifest.json"), data, 0644)
}
