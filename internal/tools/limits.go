// Copyright (C) 2025 Dyne.org foundation
// designed, written and maintained by Denis Roio <jaromil@dyne.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tools

import "sync"

// Limits configures size and traversal bounds for tool operations.
type Limits struct {
	MaxFileSizeBytes    int64
	MaxDirectoryDepth   int
	MaxDirectoryEntries int
}

const (
	defaultMaxFileSizeBytes    int64 = 10 * 1024 * 1024
	defaultMaxDirectoryDepth         = 8
	defaultMaxDirectoryEntries       = 2000
)

var (
	limitsMu      sync.RWMutex
	currentLimits = DefaultLimits()
)

// DefaultLimits returns the default resource limits for tool operations.
func DefaultLimits() Limits {
	return Limits{
		MaxFileSizeBytes:    defaultMaxFileSizeBytes,
		MaxDirectoryDepth:   defaultMaxDirectoryDepth,
		MaxDirectoryEntries: defaultMaxDirectoryEntries,
	}
}

// ConfigureLimits sets the global limits for tool operations.
func ConfigureLimits(l Limits) {
	limitsMu.Lock()
	defer limitsMu.Unlock()
	currentLimits = normalizeLimits(l)
}

func getLimits() Limits {
	limitsMu.RLock()
	defer limitsMu.RUnlock()
	return currentLimits
}

func normalizeLimits(l Limits) Limits {
	if l.MaxFileSizeBytes <= 0 {
		l.MaxFileSizeBytes = defaultMaxFileSizeBytes
	}
	if l.MaxDirectoryDepth <= 0 {
		l.MaxDirectoryDepth = defaultMaxDirectoryDepth
	}
	if l.MaxDirectoryEntries <= 0 {
		l.MaxDirectoryEntries = defaultMaxDirectoryEntries
	}
	return l
}
