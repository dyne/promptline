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

var (
	pathRulesMu     sync.RWMutex
	allowedBaseDirs []string
)

// ConfigurePathWhitelist sets optional base directories that tools may access.
func ConfigurePathWhitelist(paths []string) {
	pathRulesMu.Lock()
	defer pathRulesMu.Unlock()
	allowedBaseDirs = append([]string{}, paths...)
}

func getPathWhitelist() []string {
	pathRulesMu.RLock()
	defer pathRulesMu.RUnlock()
	return append([]string{}, allowedBaseDirs...)
}
