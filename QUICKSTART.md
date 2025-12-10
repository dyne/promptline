# Batchat - AI Chat to build batch workloads

**Note**: This project uses [bd (beads)](https://github.com/steveyegge/beads)
for issue tracking. Use `bd` commands instead of markdown TODOs.
See AGENTS.md for workflow details.

Use `bd primer` to have an overview of the task management commands.

Use `go add` to add dependencies, but do not ever run `go mod tidy` -
leave that to the user. Keep `go.sum` up to date.

Do not ever change `config.json` and assume is already setup for tests.

Do not test running the application yourself, ask the user to do it.

Do not mark tasks as completed without user confirmation.

## Project Overview

Batchat is a command-line chat interface designed to help developers
create and manage batch processing jobs for AI APIs. Rather than
executing API calls directly, Batchat guides users through defining
batch tasks and generates Python code that uses the openbatch library
to efficiently process large datasets at 50% of the cost.

The application combines:
- Interactive chat with AI assistants (via API)
- Guidance for defining batch processing tasks
- Code generation for batch submission (openbatch lib)
- Conversation context management
- Configuration-driven setup

## Technology Stack

- **Primary Language**: Go (1.22.2)
- **AI Integration**: github.com/sashabaranov/go-openai
- **Terminal UI**: tview
- **Batch programming**: python openbatch
- **Configuration**: JSON-based config files

### AI Development instructions


## Project Structure

```
batchat/
├── cmd/                    # Application entry points (missing main.go)
├── internal/               # Core application logic
│   ├── config/             # Configuration management
│   │   └── config.go
│   └── chat/               # Chat session logic
│       └── session.go
├── docs/                   # Documentation files
├── config.json.example     # Example configuration file
├── example_batch_script.py # Example of generated code
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── Makefile                # Build instructions
├── README.md               # Project documentation
└── QWEN.md                 # This file
```

## Core Components

### Configuration (`internal/config/config.go`)
- Loads configuration from JSON files
- Supports API key, base URL, model, temperature, and max tokens
- Falls back to default values (gpt-4o-mini, OpenAI API) when not specified
- Environment variable override support (OPENAI_API_KEY, DASHSCOPE_API_KEY)

### Chat Session (`internal/chat/session.go`)
- Manages conversation history with context
- Integrates with go-openai library for API communication
- Provides streaming and non-streaming response options
- Specialized code generation functionality batch scripts
- Built-in commands: /help, /clear, /history, /generate

## Building and Running

### Prerequisites
- Go 1.19+
- Python 3.8+ (for running generated scripts)
- API key for OpenAI or compatible service (like Qwen's DashScope)

### Build Commands
```bash
# Build the application
make build
# or
go build -o batchat cmd/batchat/main.go

# Install globally
make install
# or
go install cmd/batchat/main.go

# Clean build artifacts
make clean

# Format code
make fmt

# Vet code
make vet
```


## Configuration

There should be already a `config.json` file in the same directory as the executable and it should not be modified. When in need to change the running configuration then ask the human to do so.

Below some configuration examples.

For DashScope (Qwen models):
```json
{
  "api_key": "your-dashscope-api-key",
  "base_url": "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
  "model": "qwen3",
  "temperature": 0.7,
  "max_tokens": 1500
}
```

For OpenAI:
```json
{
  "api_key": "your-openai-api-key",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "temperature": 0.7,
  "max_tokens": 1500
}
```

Environment variables take precedence over config file settings.

## Usage

Run the application:
```bash
./batchat
```

Available commands during chat:
- `quit` or `exit`: Exit the application
- `clear`: Clear conversation history
- `history`: Display conversation history
- `generate`: Generate Python code for your batch task (when ready)

## Example Workflow

1. User describes a batch processing need (e.g., sentiment analysis of customer reviews)
2. AI assistant helps define the task structure and input data format
3. User requests code generation with the `/generate` command
4. Batchat produces Python code using openbatch library
5. User saves and runs the generated Python script
