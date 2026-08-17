package tools

// Deprecated test shims intentionally have no process-wide effect. Individual
// v2 tests configure their registry instance directly.
func ConfigureLimits(Limits)                    {}
func ConfigurePathWhitelist([]string)           {}
func ConfigureOutputFilters(OutputFilterConfig) {}
