package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go.temporal.io/features/harness/go/cmd"
)

const statsFileName = "test-stats.json"

func GetStats(dir string) (*cmd.Stats, error) {
	bytes, err := os.ReadFile(StatsPath(dir))
	if err != nil {
		return nil, err
	}
	var stats cmd.Stats
	if err := json.Unmarshal(bytes, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func ClearStats(dir string) error {
	return os.Remove(StatsPath(dir))
}

func StatsPath(dir string) string {
	return filepath.Join(dir, statsFileName)
}
