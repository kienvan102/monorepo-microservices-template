package entity

import "time"

// Valid values for Entry.Kind and Entry.Status.
const (
	KindJS = "js"

	StatusOK    = "ok"
	StatusError = "error"
)

type Entry struct {
	Script string `json:"script"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	File   string `json:"file,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Manifest struct {
	CapturedAt time.Time `json:"captured_at"`
	Database   string    `json:"database"`
	Collection string    `json:"collection"`
	Entries    []Entry   `json:"entries"`
}
