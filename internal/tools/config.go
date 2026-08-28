package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"promptline/internal/paths"
	"time"
)

// Config is the immutable, instance-owned authority and resource policy for a
// Registry.  Roots are capability roots: tool paths are always relative to one
// of them.  An empty Roots value grants access only to WorkingDirectory.
type Config struct {
	WorkingDirectory string
	Roots            []string
	Limits           Limits
	RateLimits       RateLimitConfig
	Timeouts         TimeoutConfig
	OutputFilters    OutputFilterConfig
	Policy           Policy
	Now              func() time.Time
	// capabilities are opened once, after configuration validation. They are
	// deliberately not exported: callers must not turn them back into host
	// pathnames and reintroduce a check-then-open race.
	capabilities []rootCapability
}

func DefaultConfig() Config {
	return Config{
		Limits:        DefaultLimits(),
		RateLimits:    DefaultRateLimitConfig(),
		Timeouts:      DefaultTimeoutConfig(),
		OutputFilters: DefaultOutputFilterConfig(),
		Policy:        DefaultPolicy(),
		Now:           time.Now,
	}
}

func normalizeConfig(config Config) (Config, error) {
	if config.WorkingDirectory == "" {
		var err error
		config.WorkingDirectory, err = os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("determine toolbox working directory: %w", err)
		}
	}
	workingDirectory, err := filepath.Abs(config.WorkingDirectory)
	if err != nil {
		return Config{}, fmt.Errorf("invalid toolbox working directory: %w", err)
	}
	workingDirectory, err = filepath.EvalSymlinks(workingDirectory)
	if err != nil {
		return Config{}, fmt.Errorf("resolve toolbox working directory: %w", err)
	}
	config.WorkingDirectory = workingDirectory
	if len(config.Roots) == 0 {
		config.Roots = []string{workingDirectory}
	} else {
		roots := make([]string, 0, len(config.Roots))
		for _, root := range config.Roots {
			if root == "" {
				return Config{}, fmt.Errorf("toolbox root cannot be empty")
			}
			abs, err := filepath.Abs(root)
			if err != nil {
				return Config{}, fmt.Errorf("invalid toolbox root: %w", err)
			}
			resolved, err := filepath.EvalSymlinks(abs)
			if err != nil {
				return Config{}, fmt.Errorf("resolve toolbox root: %w", err)
			}
			roots = append(roots, resolved)
		}
		config.Roots = roots
	}
	config.Limits = normalizeLimits(config.Limits)
	config.RateLimits = cloneRateLimits(config.RateLimits)
	config.Timeouts = cloneTimeouts(config.Timeouts)
	config.OutputFilters = normalizeOutputFilterConfig(config.OutputFilters)
	config.Policy = clonePolicy(config.Policy)
	if config.Now == nil {
		config.Now = time.Now
	}
	capabilities, err := openRootCapabilities(config.Roots, config.WorkingDirectory)
	if err != nil {
		return Config{}, err
	}
	config.capabilities = capabilities
	return config, nil
}

func clonePolicy(policy Policy) Policy {
	return Policy{Allow: cloneBoolMap(policy.Allow), Ask: cloneBoolMap(policy.Ask), Deny: cloneBoolMap(policy.Deny)}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneRateLimits(config RateLimitConfig) RateLimitConfig {
	config.PerTool = cloneIntMap(config.PerTool)
	config.Cooldowns = cloneDurationMap(config.Cooldowns)
	return config
}
func cloneIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneDurationMap(in map[string]time.Duration) map[string]time.Duration {
	out := make(map[string]time.Duration, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneTimeouts(config TimeoutConfig) TimeoutConfig {
	config.PerTool = cloneDurationMap(config.PerTool)
	return config
}

type configContextKey struct{}

func withConfig(ctx context.Context, config Config) context.Context {
	return context.WithValue(ctx, configContextKey{}, config)
}

func configFromContext(ctx context.Context) Config {
	if ctx != nil {
		if config, ok := ctx.Value(configContextKey{}).(Config); ok {
			return config
		}
	}
	return DefaultConfig()
}

func limitsFromContext(ctx context.Context) Limits { return configFromContext(ctx).Limits }

func pathAllowedByConfig(ctx context.Context, path string) bool {
	for _, root := range configFromContext(ctx).Roots {
		if paths.HasPathPrefix(path, root) {
			return true
		}
	}
	return false
}
