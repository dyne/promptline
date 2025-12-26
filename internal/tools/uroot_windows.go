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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func diskUsage(path string) (int64, int64, int64, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, 0, 0, err
	}
	volume := filepath.VolumeName(abs)
	if volume == "" {
		return 0, 0, 0, fmt.Errorf("unable to determine volume for %s", path)
	}
	if !strings.HasSuffix(volume, `\`) {
		volume += `\`
	}
	volumePtr, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return 0, 0, 0, err
	}
	var available uint64
	var total uint64
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(volumePtr, &available, &total, &free); err != nil {
		return 0, 0, 0, err
	}
	return int64(total), int64(free), int64(available), nil
}

func mkfifoPath(path string, mode uint32) error {
	return fmt.Errorf("mkfifo is not supported on windows")
}

func chmodPath(path string, mode os.FileMode) error {
	return nil
}

func ensureOwnedByCurrentUser(path string) error {
	return nil
}
