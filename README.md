# Batchat - AI Chat CLI for Coders

A minimalist command-line interface for interacting with OpenAI's API to assist with coding tasks and **batch job creation using the openbatch Python library**.

## Purpose

Batchat is designed to help developers create and manage batch processing jobs for AI APIs. Rather than executing API calls directly, Batchat guides users through defining batch tasks and then **generates Python code** that uses the [openbatch](https://github.com/daniel-gomm/openbatch) library to efficiently process large datasets at 50% of the cost.

## Features

- **Code-Focused AI Interactions**: Designed specifically for developers and coding tasks
- **Batch Job Planning**: Interactive guidance for defining batch processing tasks
- **Python Code Generation**: Automatically generates Python code using the openbatch library
- **Conversation Context**: Maintains conversation history for contextual responses
- **Configuration Driven**: Reads settings from a JSON configuration file
- **Custom Endpoints**: Supports OpenAI-compatible APIs like DashScope
- **Minimalist Design**: Clean, efficient interface without unnecessary complexity

## How It Works

1. **Chat with AI**: Describe your batch processing needs to the AI assistant
2. **Define Task**: Work with the AI to specify your input data, processing logic, and desired outputs
3. **Generate Code**: Batchat produces ready-to-run Python code using openbatch
4. **Run Batch**: Execute the generated Python script to process your data efficiently

## Installation

### Prerequisites

- Go 1.19+
- Python 3.8+
- API key for OpenAI or compatible service (like DashScope)

### Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd batchat

# Build the application
go build -o batchat cmd/batchat/main.go

# Or install it globally
go install cmd/batchat/main.go
```

## Configuration

Create a `config.json` file in the same directory as the executable:

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

Alternatively, you can set your API key using environment variables:
- For OpenAI: `OPENAI_API_KEY`
- For DashScope: `DASHSCOPE_API_KEY`

If both are provided, the environment variable takes precedence over the config file.

## Usage

Run the application:

```bash
./batchat
```

The application features a terminal-based user interface (TUI) with the following controls:
- Type your message and press Enter to send
- Press Ctrl+C to exit the application
- Press Ctrl+G to generate Python code for your batch task

Special commands (type and press Enter):
- `quit` or `exit`: Exit the application
- `clear`: Clear conversation history
- `history`: Display conversation history

## Example Workflow

```
User: I need to process customer reviews to extract sentiment
Assistant: I can help you create a batch job for sentiment analysis. What format is your data in?
User: It's in a CSV file with columns "id" and "text"
Assistant: Great! I'll help you create a batch job that processes each review and classifies sentiment as positive, neutral, or negative.
User: Yes, that sounds right
Assistant: I'll generate Python code using the openbatch library for this task.
User: generate
Assistant: Here's your Python code for batch processing customer reviews:
[Generates Python code]
```

## Project Structure

```
batchat/
├── cmd/
│   └── batchat/
│       └── main.go          # Main entry point with TUI implementation
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   └── chat/
│       └── session.go       # Chat session logic
├── config.json.example      # Example configuration file
├── go.mod                   # Go module definition
└── go.sum                   # Go module checksums
```

## License

MIT