# Homepage Structure Proposal

## Hero Section

```
PROMPTLINE

Terminal interface for streaming AI chat.
No bullshit, just code.

[Get Started] [Documentation]
```

## What It Does

```
Stream OpenAI-compatible APIs directly to your terminal.
Call functions, execute tools, pipe in batch mode.
Works over SSH, no GUI required.
```

## Quick Start

```bash
# build
make build

# run
export OPENAI_API_KEY=sk-...
./promptline

# batch mode
echo "explain this" | ./promptline -
```

## Features

**Streaming**
Real-time text streaming with full terminal scrollback.

**Tools**
AI calls functions to read/write files, execute commands.
Permission control for safety.

**Batch Mode**
Pipe queries for scripting. JSON output available.

**Portable**
Single binary. Works anywhere with a terminal.
SSH-friendly, no X11/wayland dependencies.

**Hackable**
Add your own tools in Go.
OpenAI-compatible = works with any provider.

## Architecture

```
User → Streaming Session → API
         ↓
      Tool Calls
         ↓
      System Execution
         ↓
      Response
```

Clean separation: chat session, tool registry, config, theme.

## Config

```json
{
  "api_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "tools": {
    "allow": ["read_file", "ls"],
    "require_confirmation": ["write_file", "execute_shell_command"]
  }
}
```

Environment variables override.

## Documentation

- `docs/ARCHITECTURE.md` - how it works
- `docs/TOOLS.md` - adding tools, permissions

## Requirements

- Go 1.22+
- OpenAI API key or compatible endpoint

## License

GNU AGPLv3+ - dyne.org

Free as in freedom. Fork it, hack it, use it.

---

## Design Notes

**Tone**: Technical, direct, anti-marketing. Denis Roio style.

**Visual**: Monospace, minimal. Code blocks prominent.
Terminal aesthetic - think `man` pages, not corporate landing.

**Structure**: Inverted pyramid - get to the code fast.
No "Why Choose Us" sections, no testimonials, no fluff.

**Philosophy**: 
- Tools serve users, not corporations
- Simplicity over features
- Local-first, privacy-respecting
- Hackable by design

**Copy principles**:
- Short sentences
- Active voice
- No jargon unless technical necessity
- Show, don't tell (code > words)
