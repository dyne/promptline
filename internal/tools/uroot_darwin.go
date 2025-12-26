//go:build darwin

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
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func uptimeFallbackSeconds() (float64, error) {
	boot, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0, err
	}
	bootTime := time.Unix(boot.Sec, boot.Usec*1000)
	return time.Since(bootTime).Seconds(), nil
}

func readMemInfoFallback() (map[string]string, error) {
	totalBytes, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return nil, err
	}
	pageSize, err := unix.SysctlUint64("vm.page_size")
	if err != nil {
		return nil, err
	}
	freePages, err := unix.SysctlUint64("vm.page_free_count")
	if err != nil {
		return nil, err
	}
	freeBytes := freePages * pageSize
	availablePages := freePages
	if inactivePages, err := unix.SysctlUint64("vm.page_inactive_count"); err == nil {
		availablePages += inactivePages
	}
	availableBytes := availablePages * pageSize

	entries := map[string]string{
		"MemTotal": fmt.Sprintf("%d kB", totalBytes/1024),
		"MemFree":  fmt.Sprintf("%d kB", freeBytes/1024),
	}
	if availableBytes > 0 {
		entries["MemAvailable"] = fmt.Sprintf("%d kB", availableBytes/1024)
	}
	return entries, nil
}

func listProcessesFallback(ctx context.Context, filter string, limit int) ([]processInfo, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	sort.Slice(kps, func(i, j int) bool {
		return kps[i].Proc.P_pid < kps[j].Proc.P_pid
	})

	filter = strings.ToLower(filter)
	processes := make([]processInfo, 0, limit)
	for _, kp := range kps {
		if err := ensureContext(ctx); err != nil {
			return nil, err
		}
		pid := int(kp.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		command := strings.TrimSpace(string(bytes.TrimRight(kp.Proc.P_comm[:], "\x00")))
		if command == "" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(command), filter) {
			continue
		}
		processes = append(processes, processInfo{PID: pid, Command: command})
		if len(processes) >= limit {
			break
		}
	}
	return processes, nil
}

func findProcessIDsFallback(ctx context.Context, name string) ([]int, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	sort.Slice(kps, func(i, j int) bool {
		return kps[i].Proc.P_pid < kps[j].Proc.P_pid
	})

	limits := getLimits()
	maxEntries := limits.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 2000
	}
	if len(kps) > maxEntries {
		kps = kps[:maxEntries]
	}

	nameLower := strings.ToLower(name)
	var matches []int
	for _, kp := range kps {
		if err := ensureContext(ctx); err != nil {
			return nil, err
		}
		pid := int(kp.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		command := strings.TrimSpace(string(bytes.TrimRight(kp.Proc.P_comm[:], "\x00")))
		if command == "" {
			continue
		}
		cmdName := strings.ToLower(filepath.Base(command))
		commandLower := strings.ToLower(command)
		if strings.EqualFold(command, name) ||
			cmdName == nameLower ||
			strings.HasPrefix(nameLower, cmdName) ||
			strings.HasPrefix(commandLower, nameLower) {
			matches = append(matches, pid)
		}
	}
	return matches, nil
}
