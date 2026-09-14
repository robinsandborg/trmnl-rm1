// Package storage owns file layout and JSON persistence. Callers retain DTOs
// and defaults so serialized type names and field contracts remain unchanged.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadJSON leaves dst untouched for a missing file. Decode errors may partially
// update dst; callers must discard it on error, as the legacy loaders do.
func LoadJSON(path, kind string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse %s %s: %w", kind, path, err)
	}
	return nil
}

func SaveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// AppendJSON opens the log before encoding, preserving legacy failure ordering.
func AppendJSON(path string, value any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func WriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}
