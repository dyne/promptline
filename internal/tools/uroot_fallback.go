//go:build !darwin

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
	"fmt"
)

func uptimeFallbackSeconds() (float64, error) {
	return 0, fmt.Errorf("uptime unavailable without /proc")
}

func readMemInfoFallback() (map[string]string, error) {
	return nil, fmt.Errorf("memory information unavailable without /proc")
}

func listProcessesFallback(ctx context.Context, filter string, limit int) ([]processInfo, error) {
	return nil, fmt.Errorf("process listing unavailable without /proc")
}

func findProcessIDsFallback(ctx context.Context, name string) ([]int, error) {
	return nil, fmt.Errorf("process lookup unavailable without /proc")
}
