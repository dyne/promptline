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
	"sync"
	"time"
)

// RateLimitConfig configures rate limits and cooldowns for tools.
type RateLimitConfig struct {
	DefaultPerMinute int
	PerTool          map[string]int
	Cooldowns        map[string]time.Duration
}

// DefaultRateLimitConfig returns the default rate limiting configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DefaultPerMinute: 60,
		Cooldowns: map[string]time.Duration{
			"execute_shell_command": 2 * time.Second,
		},
	}
}

type toolRateLimiter struct {
	mu          sync.Mutex
	tokens      chan struct{}
	ticker      *time.Ticker
	stop        chan struct{}
	cooldown    time.Duration
	nextAllowed time.Time
}

func newToolRateLimiter(ratePerMinute int, cooldown time.Duration) *toolRateLimiter {
	if ratePerMinute <= 0 && cooldown <= 0 {
		return nil
	}

	rl := &toolRateLimiter{
		cooldown: cooldown,
	}

	if ratePerMinute > 0 {
		interval := time.Minute / time.Duration(ratePerMinute)
		if interval <= 0 {
			interval = time.Second
		}
		burst := ratePerMinute
		if burst < 1 {
			burst = 1
		}
		rl.tokens = make(chan struct{}, burst)
		for i := 0; i < burst; i++ {
			rl.tokens <- struct{}{}
		}
		rl.ticker = time.NewTicker(interval)
		rl.stop = make(chan struct{})
		go func() {
			for {
				select {
				case <-rl.ticker.C:
					select {
					case rl.tokens <- struct{}{}:
					default:
					}
				case <-rl.stop:
					return
				}
			}
		}()
	}

	return rl
}

func (r *toolRateLimiter) Allow() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if !r.nextAllowed.IsZero() && now.Before(r.nextAllowed) {
		return fmt.Errorf("%w: retry after %s", ErrToolInCooldown, time.Until(r.nextAllowed).Round(time.Second))
	}

	if r.tokens != nil {
		select {
		case <-r.tokens:
		default:
			return ErrToolRateLimited
		}
	}

	if r.cooldown > 0 {
		r.nextAllowed = now.Add(r.cooldown)
	}

	return nil
}

func (r *toolRateLimiter) Stop() {
	if r == nil {
		return
	}
	if r.ticker != nil {
		r.ticker.Stop()
	}
	if r.stop != nil {
		close(r.stop)
	}
}
