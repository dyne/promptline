//go:build !windows

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
	"os/user"
	"strconv"
	"syscall"
)

func diskUsage(path string) (int64, int64, int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}
	size := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)
	available := int64(stat.Bavail) * int64(stat.Bsize)
	return size, free, available, nil
}

func mkfifoPath(path string, mode uint32) error {
	return syscall.Mkfifo(path, mode)
}

func chmodPath(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func ensureOwnedByCurrentUser(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("unable to determine file ownership")
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	uid, err := strconv.ParseUint(current.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to parse current user id")
	}
	if stat.Uid != uint32(uid) {
		return fmt.Errorf("refusing to change permissions on files not owned by current user")
	}
	return nil
}
