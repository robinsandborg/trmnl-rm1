// Package storage owns file layout and JSON persistence. Callers retain DTOs
// and defaults so serialized type names and field contracts remain unchanged.
package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
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

// SaveJSON durably replaces JSON and retains the previous valid bytes in .bak.
// Both files are private, same-directory atomic replacements.
func SaveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	previous, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && json.Valid(previous) && !bytes.Equal(bytes.TrimSpace(previous), []byte("null")) {
		if err := atomicWrite(path+".bak", previous); err != nil {
			return err
		}
	} else {
		if err == nil {
			if err := preserveCorrupt(path); err != nil {
				return err
			}
		}
		// A first write has its own good recovery copy. Do not replace an existing
		// backup when the primary was missing or damaged.
		if _, err := os.Stat(path + ".bak"); os.IsNotExist(err) {
			if err := atomicWrite(path+".bak", data); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return atomicWrite(path, data)
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func preserveCorrupt(path string) error {
	damaged := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return atomicWrite(damaged, data)
}

// LoadRecoverableJSON reads into a fresh value so a partial decode cannot leak.
// Invalid JSON is retained for diagnosis. A valid backup is restored durably;
// fresh controls whether damaged data without a backup may become zero state.
// Missing data without a backup is always reported as not found.
func LoadRecoverableJSON[T any](path, kind string, fresh bool, validators ...func(T) error) (T, bool, error) {
	var zero T
	read := func(name string) (T, bool, error) {
		var value T
		data, err := os.ReadFile(name)
		if os.IsNotExist(err) {
			return value, false, nil
		}
		if err != nil {
			return value, false, err
		}
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			err = errors.New("null document")
		} else {
			err = json.Unmarshal(data, &value)
		}
		if err == nil {
			for _, validate := range validators {
				if err = validate(value); err != nil {
					break
				}
			}
		}
		if err != nil {
			if preserveErr := preserveCorrupt(name); preserveErr != nil {
				return zero, false, preserveErr
			}
			return zero, true, fmt.Errorf("parse %s %s: %w", kind, name, err)
		}
		return value, true, nil
	}
	value, found, primaryErr := read(path)
	if primaryErr == nil && found {
		return value, true, nil
	}
	// Read errors (as opposed to a damaged document already preserved) must
	// never be disguised by fallback, even when a backup is available.
	var pathErr *os.PathError
	if errors.As(primaryErr, &pathErr) {
		return zero, false, primaryErr
	}
	backup, backupFound, backupErr := read(path + ".bak")
	if errors.As(backupErr, &pathErr) {
		return zero, false, backupErr
	}
	if backupErr == nil && backupFound {
		data, err := json.MarshalIndent(backup, "", "  ")
		if err != nil {
			return zero, false, err
		}
		if err := atomicWrite(path, data); err != nil {
			return zero, false, err
		}
		return backup, true, nil
	}
	if !fresh && (primaryErr != nil || backupErr != nil) {
		return zero, false, errors.Join(primaryErr, backupErr)
	}
	return zero, false, nil
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
