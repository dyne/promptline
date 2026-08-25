package mcp

import (
	"sort"

	"promptline/internal/appserver"
	"promptline/internal/tools"
)

// DynamicToolbox returns a model-facing namespace for the same tools exposed
// by the promptline-toolbox MCP server. Codex may defer ordinary MCP tools even
// when its tool-search feature is unavailable; the explicit namespace keeps
// the toolbox callable while execution still goes through mcpServer/tool/call.
func DynamicToolbox(registry *tools.Registry) appserver.DynamicToolNamespace {
	registered := registry.GetTools()
	sort.Slice(registered, func(i, j int) bool {
		return registered[i].Name() < registered[j].Name()
	})
	dynamic := make([]appserver.DynamicTool, 0, len(registered))
	for _, tool := range registered {
		dynamic = append(dynamic, appserver.DynamicTool{
			Type:        "function",
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.Parameters(),
		})
	}
	return appserver.DynamicToolNamespace{
		Type:        "namespace",
		Name:        "toolbox",
		Description: "Promptline's u-root toolbox. Use these tools for supported filesystem and system operations.",
		Tools:       dynamic,
	}
}
