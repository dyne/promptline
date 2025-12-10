package ui

import (
	"bufio"
	"os"
)

// History manages command navigation (prev/next) with an internal cursor.
type History struct {
	entries []string
	index   int
}

// LoadHistoryFromFile reads history entries from a readline history file.
func LoadHistoryFromFile(filepath string) []string {
	history := make([]string, 0)

	file, err := os.Open(filepath)
	if err != nil {
		return history
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			history = append(history, line)
		}
	}

	return history
}

// NewHistory initializes a History with existing entries.
func NewHistory(entries []string) *History {
	return &History{
		entries: entries,
		index:   -1,
	}
}

// Add appends an entry and resets navigation.
func (h *History) Add(entry string) {
	if entry == "" {
		return
	}
	h.entries = append(h.entries, entry)
	h.index = -1
}

// Prev moves backward through history. Returns entry and true if available.
func (h *History) Prev() (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.index == -1 {
		h.index = len(h.entries) - 1
	} else if h.index > 0 {
		h.index--
	}
	return h.entries[h.index], true
}

// Next moves forward through history. Returns entry (or empty when cleared) and true if movement occurred.
func (h *History) Next() (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.index == -1 {
		return "", false
	}
	if h.index < len(h.entries)-1 {
		h.index++
		return h.entries[h.index], true
	}
	h.index = -1
	return "", true
}

// Entries returns a copy of history entries.
func (h *History) Entries() []string {
	out := make([]string, len(h.entries))
	copy(out, h.entries)
	return out
}
