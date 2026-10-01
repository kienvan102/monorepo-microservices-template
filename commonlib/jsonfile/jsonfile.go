package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Writer persists a value as indented JSON. It's an interface so tests can
// swap in an in-memory writer instead of touching disk.
type Writer interface {
	WriteJSON(directory, name string, value any) error
}

func NewFileWriter() Writer { return fileWriter{} }

type fileWriter struct{}

func (fileWriter) WriteJSON(directory, name string, value any) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, name), append(data, '\n'), 0600)
}
