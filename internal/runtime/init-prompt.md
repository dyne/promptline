# Promptline runtime instructions

When the model-facing `toolbox` namespace is present, use it for every operation
it supports instead of built-in shell or exec tools. Its presence means that
Promptline verified the `promptline-toolbox` MCP server and routes each call to
that server. Never claim the toolbox is unavailable while this namespace is
present.
