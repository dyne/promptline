//go:build windows

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

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func uptimeFallbackSeconds() (float64, error) {
	return 0, fmt.Errorf("uptime unavailable on windows")
}

func readMemInfoFallback() (map[string]string, error) {
	return nil, fmt.Errorf("memory information unavailable on windows")
}

func listProcessesFallback(ctx context.Context, filter string, limit int) ([]processInfo, error) {
	if limit <= 0 {
		return nil, nil
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}

	filter = strings.ToLower(filter)
	processes := make([]processInfo, 0, limit)
	for {
		if err := ensureContext(ctx); err != nil {
			return nil, err
		}
		pid := int(entry.ProcessID)
		command := strings.TrimSpace(windows.UTF16ToString(entry.ExeFile[:]))
		if command != "" && (filter == "" || strings.Contains(strings.ToLower(command), filter)) {
			processes = append(processes, processInfo{PID: pid, Command: command})
			if len(processes) >= limit {
				break
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return processes, nil
}

func findProcessIDsFallback(ctx context.Context, name string) ([]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}

	limits := getLimits()
	maxEntries := limits.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 2000
	}

	nameLower := strings.ToLower(name)
	var matches []int
	seen := 0
	for {
		if err := ensureContext(ctx); err != nil {
			return nil, err
		}
		seen++
		if seen > maxEntries {
			break
		}
		pid := int(entry.ProcessID)
		command := strings.TrimSpace(windows.UTF16ToString(entry.ExeFile[:]))
		if command != "" {
			cmdName := strings.ToLower(filepath.Base(command))
			commandLower := strings.ToLower(command)
			if strings.EqualFold(command, name) ||
				cmdName == nameLower ||
				strings.HasPrefix(nameLower, cmdName) ||
				strings.HasPrefix(commandLower, nameLower) {
				matches = append(matches, pid)
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return matches, nil
}
