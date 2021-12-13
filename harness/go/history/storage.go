package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StoredSet contains histories by version.
type StoredSet struct {
	ByVersion map[string]Histories
}

// Storage represents a place to store histories.
type Storage struct {
	Dir  string
	Lang string
}

// Load returns all histories or nil if the directory is not present.
func (s *Storage) Load() (*StoredSet, error) {
	set := &StoredSet{ByVersion: map[string]Histories{}}
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return set, nil
	} else if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		prefix := "history." + s.Lang + "."
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		// Load the JSON and store with version
		var h Histories
		file := filepath.Join(s.Dir, entry.Name())
		if b, err := os.ReadFile(file); err != nil {
			return nil, fmt.Errorf("failed reading %v: %w", file, err)
		} else if err = json.Unmarshal(b, &h); err != nil {
			return nil, fmt.Errorf("failed unmarshaling %v: %w", file, err)
		}
		set.ByVersion[strings.TrimPrefix(strings.TrimSuffix(entry.Name(), ".json"), prefix)] = h
	}
	return set, nil
}

// Store stores the given set of histories.
func (s *Storage) Store(set *StoredSet) error {
	// Just go through overwriting not caring if files exist
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	for version, hist := range set.ByVersion {
		file := filepath.Join(s.Dir, "history."+s.Lang+"."+version+".json")
		if b, err := json.MarshalIndent(hist, "", "  "); err != nil {
			return fmt.Errorf("failed marshaling %v: %w", file, err)
		} else if err = os.WriteFile(file, b, 0644); err != nil {
			return fmt.Errorf("failed writing %v: %w", file, err)
		}
	}
	return nil
}
