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

package main

import (
	"context"
	"sync"

	"github.com/chzyer/readline"
)

type operationCanceler struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (c *operationCanceler) Set(cancel context.CancelFunc) {
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()
}

func (c *operationCanceler) Clear() {
	c.mu.Lock()
	c.cancel = nil
	c.mu.Unlock()
}

func (c *operationCanceler) Cancel() bool {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func filterInterruptRune(r rune) (rune, bool) {
	if r == readline.CharBell {
		return 0, false
	}
	return r, true
}
