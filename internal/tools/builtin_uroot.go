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
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"promptline/internal/paths"

	"github.com/u-root/u-root/pkg/core"
	corebase64 "github.com/u-root/u-root/pkg/core/base64"
	corecat "github.com/u-root/u-root/pkg/core/cat"
	corecp "github.com/u-root/u-root/pkg/core/cp"
	corels "github.com/u-root/u-root/pkg/core/ls"
	coremkdir "github.com/u-root/u-root/pkg/core/mkdir"
	coremv "github.com/u-root/u-root/pkg/core/mv"
	corerm "github.com/u-root/u-root/pkg/core/rm"
	coreshasum "github.com/u-root/u-root/pkg/core/shasum"
	coretouch "github.com/u-root/u-root/pkg/core/touch"
)

const urootToolVersion = "1.0.0"

type urootCommand func(ctx context.Context, args []string) (string, error)

func registerURootTools(r *Registry) {
	register := func(tool Tool) {
		if err := r.RegisterTool(tool); err != nil {
			panic(err)
		}
	}

	register(&ToolDefinition{
		NameValue:        "ls",
		DescriptionValue: "List directory contents",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path to list (default: current directory)",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "List directories recursively",
				},
				"show_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files",
				},
			},
		},
		ExecuteFunc:  executeLs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "cat",
		DescriptionValue: "Concatenate and print files",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to concatenate",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to concatenate",
				},
			},
		},
		ExecuteFunc:  wrapURootCommand(buildCatArgs, runCat),
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "cp",
		DescriptionValue: "Copy files and directories",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sources": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Source file or directory paths",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "Destination path",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Copy directories recursively",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Overwrite existing files",
				},
				"no_follow_symlinks": map[string]interface{}{
					"type":        "boolean",
					"description": "Copy symlink itself instead of target",
				},
			},
			"required": []string{"sources", "destination"},
		},
		ExecuteFunc:  wrapURootCommand(buildCopyArgs, runCopy),
		ValidateFunc: validateRequiredStrings([]string{"destination"}, []string{"sources"}),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "mv",
		DescriptionValue: "Move or rename files and directories",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sources": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Source file or directory paths",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "Destination path",
				},
				"update": map[string]interface{}{
					"type":        "boolean",
					"description": "Move only when source is newer or destination is missing",
				},
				"no_clobber": map[string]interface{}{
					"type":        "boolean",
					"description": "Do not overwrite existing files",
				},
			},
			"required": []string{"sources", "destination"},
		},
		ExecuteFunc:  wrapURootCommand(buildMoveArgs, runMove),
		ValidateFunc: validateRequiredStrings([]string{"destination"}, []string{"sources"}),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "rm",
		DescriptionValue: "Remove files or directories",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Paths to remove",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single path to remove",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Remove directories recursively",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Ignore nonexistent files",
				},
			},
		},
		ExecuteFunc:  wrapURootCommand(buildRemoveArgs, runRemove),
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "ln",
		DescriptionValue: "Create a link to a file",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Existing target path",
				},
				"link_path": map[string]interface{}{
					"type":        "string",
					"description": "New link path",
				},
				"symbolic": map[string]interface{}{
					"type":        "boolean",
					"description": "Create a symbolic link instead of hard link",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Remove existing destination before linking",
				},
			},
			"required": []string{"target", "link_path"},
		},
		ExecuteFunc:  linkPath,
		ValidateFunc: validateRequiredStrings([]string{"target", "link_path"}, nil),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "touch",
		DescriptionValue: "Create files or update timestamps",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to touch",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to touch",
				},
				"access": map[string]interface{}{
					"type":        "boolean",
					"description": "Change access time only",
				},
				"modification": map[string]interface{}{
					"type":        "boolean",
					"description": "Change modification time only",
				},
				"no_create": map[string]interface{}{
					"type":        "boolean",
					"description": "Do not create files if they do not exist",
				},
				"datetime": map[string]interface{}{
					"type":        "string",
					"description": "RFC3339 timestamp to apply",
				},
			},
		},
		ExecuteFunc:  wrapURootCommand(buildTouchArgs, runTouch),
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "truncate",
		DescriptionValue: "Shrink or extend a file to a specified size",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to truncate",
				},
				"size": map[string]interface{}{
					"type":        "number",
					"description": "Size in bytes",
				},
				"no_create": map[string]interface{}{
					"type":        "boolean",
					"description": "Do not create file if missing",
				},
			},
			"required": []string{"path", "size"},
		},
		ExecuteFunc:  truncateFile,
		ValidateFunc: validateTruncateArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "readlink",
		DescriptionValue: "Print resolved symbolic link target",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Symlink path to read",
				},
				"follow": map[string]interface{}{
					"type":        "boolean",
					"description": "Resolve symlinks recursively",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  readLinkPath,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "realpath",
		DescriptionValue: "Print absolute path with symlinks resolved",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to resolve",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  realpathPath,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "grep",
		DescriptionValue: "Search text patterns in files",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Pattern to search for (regular expression)",
				},
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to search",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to search",
				},
				"ignore_case": map[string]interface{}{
					"type":        "boolean",
					"description": "Case-insensitive matching",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Search directories recursively",
				},
				"show_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files when searching directories",
				},
				"invert": map[string]interface{}{
					"type":        "boolean",
					"description": "Select non-matching lines",
				},
				"max_matches": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of matches to return",
				},
			},
			"required": []string{"pattern"},
		},
		ExecuteFunc:  grepText,
		ValidateFunc: validateGrepArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "head",
		DescriptionValue: "Output the first part of files",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to read",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to read",
				},
				"lines": map[string]interface{}{
					"type":        "number",
					"description": "Number of lines to return (default: 10)",
				},
			},
		},
		ExecuteFunc:  headText,
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "tail",
		DescriptionValue: "Output the last part of files",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to read",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to read",
				},
				"lines": map[string]interface{}{
					"type":        "number",
					"description": "Number of lines to return (default: 10)",
				},
			},
		},
		ExecuteFunc:  tailText,
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "sort",
		DescriptionValue: "Sort lines of text",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to sort",
				},
				"reverse": map[string]interface{}{
					"type":        "boolean",
					"description": "Reverse sort order",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  sortText,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "uniq",
		DescriptionValue: "Report or omit repeated lines",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to process",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  uniqText,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "wc",
		DescriptionValue: "Word, line, and byte count",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to count",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to count",
				},
			},
		},
		ExecuteFunc:  wordCount,
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "tr",
		DescriptionValue: "Translate characters",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Characters to replace",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "Replacement characters",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to process",
				},
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Inline text to process (if path not provided)",
				},
			},
			"required": []string{"from", "to"},
		},
		ExecuteFunc:  translateText,
		ValidateFunc: validateTranslateArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "tee",
		DescriptionValue: "Write input to files and standard output",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write",
				},
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to write",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to write",
				},
			},
			"required": []string{"content"},
		},
		ExecuteFunc:  teeText,
		ValidateFunc: validateTeeArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "comm",
		DescriptionValue: "Compare sorted files line by line",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path1": map[string]interface{}{
					"type":        "string",
					"description": "First file path",
				},
				"path2": map[string]interface{}{
					"type":        "string",
					"description": "Second file path",
				},
			},
			"required": []string{"path1", "path2"},
		},
		ExecuteFunc:  compareFiles,
		ValidateFunc: validateCommArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "strings",
		DescriptionValue: "Print printable character sequences",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to scan",
				},
				"min_length": map[string]interface{}{
					"type":        "number",
					"description": "Minimum string length (default: 4)",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  stringsText,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "more",
		DescriptionValue: "Page through text",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to read",
				},
				"lines": map[string]interface{}{
					"type":        "number",
					"description": "Number of lines to return (default: 40)",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  moreText,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "hexdump",
		DescriptionValue: "Display file contents in hexadecimal",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to dump",
				},
				"max_bytes": map[string]interface{}{
					"type":        "number",
					"description": "Maximum bytes to display (default: 512)",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  hexDump,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "cmp",
		DescriptionValue: "Compare two files byte by byte",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path1": map[string]interface{}{
					"type":        "string",
					"description": "First file path",
				},
				"path2": map[string]interface{}{
					"type":        "string",
					"description": "Second file path",
				},
			},
			"required": []string{"path1", "path2"},
		},
		ExecuteFunc:  compareBytes,
		ValidateFunc: validateCommArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "md5sum",
		DescriptionValue: "Compute MD5 checksums",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to hash",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to hash",
				},
			},
		},
		ExecuteFunc:  md5Sum,
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "shasum",
		DescriptionValue: "Compute SHA checksums",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "File paths to hash",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single file path to hash",
				},
				"algorithm": map[string]interface{}{
					"type":        "number",
					"description": "SHA algorithm (1, 256, or 512)",
				},
			},
		},
		ExecuteFunc:  shaSum,
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "base64",
		DescriptionValue: "Base64 encode or decode files",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to encode or decode",
				},
				"decode": map[string]interface{}{
					"type":        "boolean",
					"description": "Decode base64 input",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  base64Tool,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "mkdir",
		DescriptionValue: "Create directories",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Directory paths to create",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Single directory path to create",
				},
				"parents": map[string]interface{}{
					"type":        "boolean",
					"description": "Create parent directories as needed",
				},
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "Octal permission mode (e.g., 755)",
				},
			},
		},
		ExecuteFunc:  wrapURootCommand(buildMkdirArgs, runMkdir),
		ValidateFunc: validatePathsArg("paths", "path"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "pwd",
		DescriptionValue: "Print the working directory",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  printWorkingDirectory,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "dirname",
		DescriptionValue: "Strip last component from path",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to evaluate",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  dirNamePath,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "basename",
		DescriptionValue: "Strip directory from filename",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to evaluate",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  baseNamePath,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "uname",
		DescriptionValue: "Print system information",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  unameTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "hostname",
		DescriptionValue: "Print system hostname",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  hostnameTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "uptime",
		DescriptionValue: "Show how long the system has been running",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  uptimeTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "free",
		DescriptionValue: "Display memory usage",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  freeTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "df",
		DescriptionValue: "Report filesystem disk space usage",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to inspect (default: current directory)",
				},
			},
		},
		ExecuteFunc:  dfTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "du",
		DescriptionValue: "Estimate file space usage",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to inspect (default: current directory)",
				},
				"max_depth": map[string]interface{}{
					"type":        "number",
					"description": "Maximum depth to traverse",
				},
			},
		},
		ExecuteFunc:  duTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "ps",
		DescriptionValue: "Report process status",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Filter processes by substring match",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of processes to return",
				},
			},
		},
		ExecuteFunc:  psTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "pidof",
		DescriptionValue: "Find process IDs by name",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Process name to match",
				},
			},
			"required": []string{"name"},
		},
		ExecuteFunc:  pidofTool,
		ValidateFunc: RequireNonEmptyArg("name", "missing or invalid 'name' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "id",
		DescriptionValue: "Print user identity",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type":        "string",
					"description": "User name to inspect (default: current user)",
				},
			},
		},
		ExecuteFunc:  idTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "echo",
		DescriptionValue: "Display text",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Text to echo",
				},
				"parts": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Text parts to join with spaces",
				},
			},
		},
		ExecuteFunc:  echoTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "seq",
		DescriptionValue: "Print sequence of numbers",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"start": map[string]interface{}{
					"type":        "number",
					"description": "Start value (default: 1 if end specified)",
				},
				"end": map[string]interface{}{
					"type":        "number",
					"description": "End value",
				},
				"step": map[string]interface{}{
					"type":        "number",
					"description": "Step value (default: 1)",
				},
			},
		},
		ExecuteFunc:  seqTool,
		ValidateFunc: validateSeqArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "printenv",
		DescriptionValue: "Print environment variables",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Specific environment variable to print",
				},
			},
		},
		ExecuteFunc:  printenvTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "tty",
		DescriptionValue: "Print terminal name",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  ttyTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "which",
		DescriptionValue: "Locate a command in PATH",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Command name to locate",
				},
			},
			"required": []string{"name"},
		},
		ExecuteFunc:  whichTool,
		ValidateFunc: RequireNonEmptyArg("name", "missing or invalid 'name' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "mkfifo",
		DescriptionValue: "Create a named pipe",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "FIFO path to create",
				},
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "Octal permission mode (default: 600)",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  mkfifoTool,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "mktemp",
		DescriptionValue: "Create a temporary file or directory",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dir": map[string]interface{}{
					"type":        "boolean",
					"description": "Create a temporary directory instead of a file",
				},
				"prefix": map[string]interface{}{
					"type":        "string",
					"description": "Prefix for the temporary name",
				},
			},
		},
		ExecuteFunc:  mktempTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "find",
		DescriptionValue: "Search for files",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Root path to search (default: current directory)",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern to match file names",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by type: file or dir",
				},
				"max_depth": map[string]interface{}{
					"type":        "number",
					"description": "Maximum depth to traverse",
				},
				"show_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden entries",
				},
			},
		},
		ExecuteFunc:  findTool,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "chmod",
		DescriptionValue: "Change file permissions",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to modify",
				},
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "Octal permission mode (e.g., 644)",
				},
			},
			"required": []string{"path", "mode"},
		},
		ExecuteFunc:  chmodTool,
		ValidateFunc: validateChmodArgs,
		VersionValue: urootToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "date",
		DescriptionValue: "Display date and time",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Go time layout or 'unix' (default: RFC3339)",
				},
			},
		},
		ExecuteFunc:  dateTool,
		VersionValue: urootToolVersion,
	})
}

func wrapURootCommand(buildArgs func(map[string]interface{}) ([]string, error), run urootCommand) ExecutorFunc {
	return func(ctx context.Context, args map[string]interface{}) (string, error) {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		if buildArgs == nil {
			return "", fmt.Errorf("missing u-root argument builder")
		}
		if run == nil {
			return "", fmt.Errorf("missing u-root command runner")
		}
		cmdArgs, err := buildArgs(args)
		if err != nil {
			return "", err
		}
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		return run(ctx, cmdArgs)
	}
}

func resolveToolPath(path string) (string, error) {
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return "", err
	}

	baseResolved, err := resolveBaseDir()
	if err != nil {
		return "", err
	}

	if filepath.IsAbs(path) {
		resolved, err := ensureResolvedPathWithinBase(path, baseResolved)
		if err != nil {
			return "", err
		}
		return resolved, nil
	}

	return resolvePathWithinBase(path, baseResolved)
}

func resolveToolPathNoSymlink(path string) (string, error) {
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return "", err
	}

	baseResolved, err := resolveBaseDir()
	if err != nil {
		return "", err
	}

	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Clean(filepath.Join(baseResolved, path))
	}

	parent := filepath.Dir(abs)
	parentResolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !paths.HasPathPrefix(parentResolved, baseResolved) {
		return "", fmt.Errorf("path escapes working directory")
	}
	resolved := filepath.Join(parentResolved, filepath.Base(abs))

	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(resolved, dangerous) {
			return "", fmt.Errorf("access to %s is restricted for security", dangerous)
		}
	}
	if err := validatePathWhitelist(resolved, baseResolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func resolveAbsolutePath(path, baseResolved string) (string, error) {
	abs := filepath.Clean(path)
	resolved, err := paths.ResolveSymlinkedPath(abs, baseResolved)
	if err != nil {
		return "", err
	}
	if !paths.HasPathPrefix(resolved, baseResolved) {
		return "", fmt.Errorf("path escapes working directory")
	}
	return resolved, nil
}

func ensureResolvedPathWithinBase(path, baseResolved string) (string, error) {
	resolved, err := resolveAbsolutePath(path, baseResolved)
	if err != nil {
		return "", err
	}
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(resolved, dangerous) {
			return "", fmt.Errorf("access to %s is restricted for security", dangerous)
		}
	}
	if err := validatePathWhitelist(resolved, baseResolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func resolveBaseDir() (string, error) {
	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}
	baseAbs, err := filepath.Abs(workdir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %v", err)
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %v", err)
	}
	return baseResolved, nil
}

func runCoreCommand(ctx context.Context, cmd core.Command, args []string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &stderr)

	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}
	cmd.SetWorkingDir(workdir)

	if err := cmd.RunContext(ctx, args...); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%v: %s", err, errMsg)
		}
		return "", err
	}

	return stdout.String(), nil
}

func executeLs(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path := getPathArg(args)
	recursive := getBoolArg(args, "recursive")
	showHidden := getBoolArg(args, "show_hidden")

	resolved, err := resolveListPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("path not found: %v", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path '%s' is not a directory", resolved)
	}

	limits := getLimits()
	var limiter strings.Builder
	if recursive {
		if err := walkDirectory(ctx, resolved, showHidden, limits, &limiter); err != nil {
			return "", err
		}
	} else {
		if err := listDirectoryNonRecursive(ctx, resolved, showHidden, limits, &limiter); err != nil {
			return "", err
		}
	}

	var cmdArgs []string
	if showHidden {
		cmdArgs = append(cmdArgs, "-a")
	}
	if recursive {
		cmdArgs = append(cmdArgs, "-R")
	}
	if path != "" {
		cmdArgs = append(cmdArgs, path)
	}
	output, err := runCoreCommand(ctx, corels.New(), cmdArgs)
	if err != nil {
		return "", err
	}
	if !showHidden {
		output = filterHiddenOutput(output)
	}
	if strings.TrimSpace(output) == "" {
		return "Directory is empty", nil
	}
	return output, nil
}

func buildCatArgs(args map[string]interface{}) ([]string, error) {
	paths, err := extractPaths(args, "paths", "path")
	if err != nil {
		return nil, err
	}
	return resolveToolPaths(paths)
}

func runCat(ctx context.Context, args []string) (string, error) {
	limits := getLimits()
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("path '%s' is a directory", path)
		}
		if info.Size() > limits.MaxFileSizeBytes {
			return "", fmt.Errorf("file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
		}
	}
	return runCoreCommand(ctx, corecat.New(), args)
}

func buildCopyArgs(args map[string]interface{}) ([]string, error) {
	srcs, err := extractStringSliceArg(args, "sources")
	if err != nil {
		return nil, err
	}
	dest, err := extractStringArg(args, "destination")
	if err != nil {
		return nil, err
	}

	resolvedSources, err := resolveToolPaths(srcs)
	if err != nil {
		return nil, err
	}
	resolvedDest, err := resolveToolPath(dest)
	if err != nil {
		return nil, err
	}

	limits := getLimits()
	for _, source := range resolvedSources {
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		if info.IsDir() && !getBoolArg(args, "recursive") {
			return nil, fmt.Errorf("source '%s' is a directory (set recursive to true)", source)
		}
		if !info.IsDir() && info.Size() > limits.MaxFileSizeBytes {
			return nil, fmt.Errorf("file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
		}
	}

	var cmdArgs []string
	if getBoolArg(args, "recursive") {
		cmdArgs = append(cmdArgs, "-r")
	}
	if getBoolArg(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}
	if getBoolArg(args, "no_follow_symlinks") {
		cmdArgs = append(cmdArgs, "-P")
	}
	cmdArgs = append(cmdArgs, resolvedSources...)
	cmdArgs = append(cmdArgs, resolvedDest)
	return cmdArgs, nil
}

func runCopy(ctx context.Context, args []string) (string, error) {
	return runCoreCommand(ctx, corecp.New(), args)
}

func buildMoveArgs(args map[string]interface{}) ([]string, error) {
	srcs, err := extractStringSliceArg(args, "sources")
	if err != nil {
		return nil, err
	}
	dest, err := extractStringArg(args, "destination")
	if err != nil {
		return nil, err
	}

	resolvedSources, err := resolveToolPaths(srcs)
	if err != nil {
		return nil, err
	}
	resolvedDest, err := resolveToolPath(dest)
	if err != nil {
		return nil, err
	}

	var cmdArgs []string
	if getBoolArg(args, "update") {
		cmdArgs = append(cmdArgs, "-u")
	}
	if getBoolArg(args, "no_clobber") {
		cmdArgs = append(cmdArgs, "-n")
	}
	cmdArgs = append(cmdArgs, resolvedSources...)
	cmdArgs = append(cmdArgs, resolvedDest)
	return cmdArgs, nil
}

func runMove(ctx context.Context, args []string) (string, error) {
	return runCoreCommand(ctx, coremv.New(), args)
}

func buildRemoveArgs(args map[string]interface{}) ([]string, error) {
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return nil, err
	}
	resolved, err := resolveToolPaths(pathsArg)
	if err != nil {
		return nil, err
	}

	var cmdArgs []string
	if getBoolArg(args, "recursive") {
		cmdArgs = append(cmdArgs, "-r")
	}
	if getBoolArg(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, resolved...)
	return cmdArgs, nil
}

func runRemove(ctx context.Context, args []string) (string, error) {
	return runCoreCommand(ctx, corerm.New(), args)
}

func buildTouchArgs(args map[string]interface{}) ([]string, error) {
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return nil, err
	}
	resolved, err := resolveToolPaths(pathsArg)
	if err != nil {
		return nil, err
	}

	var cmdArgs []string
	if getBoolArg(args, "access") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if getBoolArg(args, "modification") {
		cmdArgs = append(cmdArgs, "-m")
	}
	if getBoolArg(args, "no_create") {
		cmdArgs = append(cmdArgs, "-c")
	}
	if datetime, ok := getStringLike(args["datetime"]); ok {
		cmdArgs = append(cmdArgs, "-d", datetime)
	}
	cmdArgs = append(cmdArgs, resolved...)
	return cmdArgs, nil
}

func runTouch(ctx context.Context, args []string) (string, error) {
	return runCoreCommand(ctx, coretouch.New(), args)
}

func buildMkdirArgs(args map[string]interface{}) ([]string, error) {
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return nil, err
	}
	resolved, err := resolveToolPaths(pathsArg)
	if err != nil {
		return nil, err
	}

	var cmdArgs []string
	if getBoolArg(args, "parents") {
		cmdArgs = append(cmdArgs, "-p")
	}
	if mode, ok := getStringLike(args["mode"]); ok {
		if err := validateMode(mode); err != nil {
			return nil, err
		}
		cmdArgs = append(cmdArgs, "-m", mode)
	}
	cmdArgs = append(cmdArgs, resolved...)
	return cmdArgs, nil
}

func runMkdir(ctx context.Context, args []string) (string, error) {
	return runCoreCommand(ctx, coremkdir.New(), args)
}

func grepText(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	pattern, err := extractStringArg(args, "pattern")
	if err != nil {
		return "", err
	}
	paths, err := extractPaths(args, "paths", "path")
	if err != nil {
		return "", err
	}

	recursive := getBoolArg(args, "recursive")
	showHidden := getBoolArg(args, "show_hidden")
	files, err := collectGrepFiles(ctx, paths, recursive, showHidden)
	if err != nil {
		return "", err
	}

	if getBoolArg(args, "ignore_case") {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	maxMatches, err := extractIntArg(args, "max_matches", 1000)
	if err != nil {
		return "", err
	}
	if maxMatches <= 0 {
		return "", fmt.Errorf("max_matches must be positive")
	}

	invert := getBoolArg(args, "invert")
	var output []string
	matchCount := 0
	multiFile := len(files) > 1

	for _, file := range files {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		data, err := readFileLimited(file.Path, true)
		if err != nil {
			return "", err
		}
		if !isTextContent(data) {
			if file.FromDir {
				continue
			}
			return "", fmt.Errorf("file appears to be binary; tool supports text only")
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			match := re.MatchString(line)
			if invert {
				match = !match
			}
			if match {
				if multiFile {
					output = append(output, fmt.Sprintf("%s:%s", file.Display, line))
				} else {
					output = append(output, line)
				}
				matchCount++
				if matchCount >= maxMatches {
					return strings.Join(output, "\n"), nil
				}
			}
		}
	}

	return strings.Join(output, "\n"), nil
}

type grepFile struct {
	Path    string
	Display string
	FromDir bool
}

type walkEntry struct {
	Path  string
	Rel   string
	IsDir bool
}

type walkOptions struct {
	maxDepth    int
	maxEntries  int
	showHidden  bool
	pattern     string
	typeFilter  string
	regularOnly bool
}

func walkDirEntries(ctx context.Context, root string, opts walkOptions) ([]walkEntry, error) {
	maxDepth := opts.maxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	matches := make([]walkEntry, 0, 64)
	entries := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ensureContext(ctx); err != nil {
			return err
		}
		depth := relativeDepth(root, path)
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." && !opts.showHidden && containsHiddenSegment(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if opts.pattern != "" {
			match, err := filepath.Match(opts.pattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if !match {
				return nil
			}
		}
		if opts.typeFilter == "file" && d.IsDir() {
			return nil
		}
		if opts.typeFilter == "dir" && !d.IsDir() {
			return nil
		}
		if opts.regularOnly {
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
		}
		matches = append(matches, walkEntry{Path: path, Rel: rel, IsDir: d.IsDir()})
		entries++
		if opts.maxEntries > 0 && entries >= opts.maxEntries {
			return fmt.Errorf("directory entry limit exceeded")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func collectGrepFiles(ctx context.Context, inputPaths []string, recursive bool, showHidden bool) ([]grepFile, error) {
	baseResolved, err := resolveBaseDir()
	if err != nil {
		return nil, err
	}
	expandedInputs := make([]string, 0, len(inputPaths))
	expandedResolved := make([]string, 0, len(inputPaths))
	for _, input := range inputPaths {
		if hasGlobMeta(input) {
			matches, err := expandGlobPattern(baseResolved, input)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("pattern %q matched no files", input)
			}
			for _, match := range matches {
				resolvedMatch, err := resolveToolPath(match)
				if err != nil {
					return nil, err
				}
				display := match
				if !filepath.IsAbs(input) {
					rel, err := filepath.Rel(baseResolved, resolvedMatch)
					if err == nil {
						display = rel
					}
				}
				expandedInputs = append(expandedInputs, display)
				expandedResolved = append(expandedResolved, resolvedMatch)
			}
			continue
		}
		resolved, err := resolveToolPath(input)
		if err != nil {
			return nil, err
		}
		expandedInputs = append(expandedInputs, input)
		expandedResolved = append(expandedResolved, resolved)
	}
	files := make([]grepFile, 0, len(expandedResolved))
	limits := getLimits()
	for i, resolved := range expandedResolved {
		input := expandedInputs[i]
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, grepFile{Path: resolved, Display: input})
			continue
		}
		maxDepth := 1
		if recursive {
			maxDepth = limits.MaxDirectoryDepth
		}
		entries, err := walkDirEntries(ctx, resolved, walkOptions{
			maxDepth:    maxDepth,
			maxEntries:  limits.MaxDirectoryEntries,
			showHidden:  showHidden,
			typeFilter:  "file",
			regularOnly: true,
		})
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			display := input
			if entry.Rel != "." {
				display = filepath.Join(display, entry.Rel)
			}
			files = append(files, grepFile{
				Path:    entry.Path,
				Display: display,
				FromDir: true,
			})
		}
	}
	return files, nil
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func expandGlobPattern(baseResolved, input string) ([]string, error) {
	if err := paths.ValidatePathString(input, maxPathLength); err != nil {
		return nil, err
	}
	var pattern string
	if filepath.IsAbs(input) {
		pattern = filepath.Clean(input)
	} else {
		pattern = filepath.Clean(filepath.Join(baseResolved, input))
	}
	if !paths.HasPathPrefix(pattern, baseResolved) {
		return nil, fmt.Errorf("path escapes working directory")
	}
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(pattern, dangerous) {
			return nil, fmt.Errorf("access to %s is restricted for security", dangerous)
		}
	}
	return filepath.Glob(pattern)
}

func headText(ctx context.Context, args map[string]interface{}) (string, error) {
	return readHeadTail(ctx, args, true, 10)
}

func tailText(ctx context.Context, args map[string]interface{}) (string, error) {
	return readHeadTail(ctx, args, false, 10)
}

func sortText(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	lines, err := readTextLines(resolved)
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	if getBoolArg(args, "reverse") {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	return strings.Join(lines, "\n"), nil
}

func uniqText(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	lines, err := readTextLines(resolved)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}
	var output []string
	previous := lines[0]
	output = append(output, previous)
	for _, line := range lines[1:] {
		if line == previous {
			continue
		}
		previous = line
		output = append(output, line)
	}
	return strings.Join(output, "\n"), nil
}

func wordCount(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	paths, err := extractPaths(args, "paths", "path")
	if err != nil {
		return "", err
	}
	resolvedPaths, err := resolveToolPaths(paths)
	if err != nil {
		return "", err
	}
	var output []string
	for idx, path := range resolvedPaths {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		data, err := readFileLimited(path, false)
		if err != nil {
			return "", err
		}
		lines := countLines(data)
		words := countWords(data)
		bytesCount := len(data)
		if len(resolvedPaths) > 1 {
			output = append(output, fmt.Sprintf("%d %d %d %s", lines, words, bytesCount, paths[idx]))
		} else {
			output = append(output, fmt.Sprintf("%d %d %d", lines, words, bytesCount))
		}
	}
	return strings.Join(output, "\n"), nil
}

func translateText(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	from, err := extractStringArg(args, "from")
	if err != nil {
		return "", err
	}
	to, err := extractStringArg(args, "to")
	if err != nil {
		return "", err
	}
	if from == "" || to == "" {
		return "", fmt.Errorf("from and to must be non-empty")
	}

	var input string
	if path, ok := getStringLike(args["path"]); ok {
		resolved, err := resolveToolPath(path)
		if err != nil {
			return "", err
		}
		data, err := readFileLimited(resolved, false)
		if err != nil {
			return "", err
		}
		input = string(data)
	} else if inline, ok := getStringLike(args["input"]); ok {
		input = inline
	} else {
		return "", fmt.Errorf("missing or invalid 'path' or 'input' parameter")
	}

	mapping := buildTranslationMap(from, to)
	var output strings.Builder
	for _, r := range input {
		if replacement, ok := mapping[r]; ok {
			output.WriteRune(replacement)
		} else {
			output.WriteRune(r)
		}
	}
	return output.String(), nil
}

func teeText(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	content, err := extractStringArg(args, "content")
	if err != nil {
		return "", err
	}
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return "", err
	}
	resolvedPaths, err := resolveToolPaths(pathsArg)
	if err != nil {
		return "", err
	}
	limits := getLimits()
	if limits.MaxFileSizeBytes > 0 && int64(len(content)) > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("content exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}
	for _, path := range resolvedPaths {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return content, nil
}

func compareFiles(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path1, err := extractStringArg(args, "path1")
	if err != nil {
		return "", err
	}
	path2, err := extractStringArg(args, "path2")
	if err != nil {
		return "", err
	}
	resolved1, err := resolveToolPath(path1)
	if err != nil {
		return "", err
	}
	resolved2, err := resolveToolPath(path2)
	if err != nil {
		return "", err
	}
	lines1, err := readTextLines(resolved1)
	if err != nil {
		return "", err
	}
	lines2, err := readTextLines(resolved2)
	if err != nil {
		return "", err
	}

	var output []string
	i, j := 0, 0
	for i < len(lines1) || j < len(lines2) {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		switch {
		case i >= len(lines1):
			output = append(output, "\t"+lines2[j])
			j++
		case j >= len(lines2):
			output = append(output, lines1[i])
			i++
		case lines1[i] == lines2[j]:
			output = append(output, "\t\t"+lines1[i])
			i++
			j++
		case lines1[i] < lines2[j]:
			output = append(output, lines1[i])
			i++
		default:
			output = append(output, "\t"+lines2[j])
			j++
		}
	}
	return strings.Join(output, "\n"), nil
}

func stringsText(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	minLength, err := extractIntArg(args, "min_length", 4)
	if err != nil {
		return "", err
	}
	if minLength <= 0 {
		return "", fmt.Errorf("min_length must be positive")
	}
	data, err := readFileLimited(resolved, true)
	if err != nil {
		return "", err
	}
	var output []string
	var current []byte
	for _, b := range data {
		if b >= 32 && b <= 126 {
			current = append(current, b)
			continue
		}
		if len(current) >= minLength {
			output = append(output, string(current))
		}
		current = current[:0]
	}
	if len(current) >= minLength {
		output = append(output, string(current))
	}
	return strings.Join(output, "\n"), nil
}

func moreText(ctx context.Context, args map[string]interface{}) (string, error) {
	return readHeadTail(ctx, args, true, 40)
}

func hexDump(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	maxBytes, err := extractIntArg(args, "max_bytes", 512)
	if err != nil {
		return "", err
	}
	if maxBytes <= 0 {
		return "", fmt.Errorf("max_bytes must be positive")
	}
	data, err := readFileLimited(resolved, true)
	if err != nil {
		return "", err
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}

	var output strings.Builder
	for offset := 0; offset < len(data); offset += 16 {
		end := offset + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		var hexParts []string
		var ascii []byte
		for i := 0; i < 16; i++ {
			if i < len(chunk) {
				b := chunk[i]
				hexParts = append(hexParts, fmt.Sprintf("%02x", b))
				if b >= 32 && b <= 126 {
					ascii = append(ascii, b)
				} else {
					ascii = append(ascii, '.')
				}
			} else {
				hexParts = append(hexParts, "  ")
				ascii = append(ascii, ' ')
			}
		}
		output.WriteString(fmt.Sprintf("%08x  %s  |%s|\n", offset, strings.Join(hexParts, " "), string(ascii)))
	}
	return strings.TrimRight(output.String(), "\n"), nil
}

func compareBytes(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path1, err := extractStringArg(args, "path1")
	if err != nil {
		return "", err
	}
	path2, err := extractStringArg(args, "path2")
	if err != nil {
		return "", err
	}
	resolved1, err := resolveToolPath(path1)
	if err != nil {
		return "", err
	}
	resolved2, err := resolveToolPath(path2)
	if err != nil {
		return "", err
	}
	if err := ensureFileWithinLimit(resolved1); err != nil {
		return "", err
	}
	if err := ensureFileWithinLimit(resolved2); err != nil {
		return "", err
	}

	file1, err := os.Open(resolved1)
	if err != nil {
		return "", err
	}
	defer file1.Close()

	file2, err := os.Open(resolved2)
	if err != nil {
		return "", err
	}
	defer file2.Close()

	buf1 := make([]byte, 4096)
	buf2 := make([]byte, 4096)
	var offset int64

	for {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		n1, err1 := file1.Read(buf1)
		n2, err2 := file2.Read(buf2)

		min := n1
		if n2 < min {
			min = n2
		}
		for i := 0; i < min; i++ {
			if buf1[i] != buf2[i] {
				return fmt.Sprintf("Files differ at byte %d", offset+int64(i)+1), nil
			}
		}
		if n1 != n2 {
			return fmt.Sprintf("Files differ at byte %d", offset+int64(min)+1), nil
		}

		if err1 == io.EOF && err2 == io.EOF {
			return "Files are identical", nil
		}
		if err1 != nil && err1 != io.EOF {
			return "", err1
		}
		if err2 != nil && err2 != io.EOF {
			return "", err2
		}
		offset += int64(n1)
	}
}

func md5Sum(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return "", err
	}
	resolvedPaths, err := resolveToolPaths(pathsArg)
	if err != nil {
		return "", err
	}
	var output []string
	for idx, path := range resolvedPaths {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		if err := ensureFileWithinLimit(path); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			file.Close()
			return "", err
		}
		file.Close()
		sum := hex.EncodeToString(hash.Sum(nil))
		if len(resolvedPaths) > 1 {
			output = append(output, fmt.Sprintf("%s  %s", sum, pathsArg[idx]))
		} else {
			output = append(output, fmt.Sprintf("%s  %s", sum, pathsArg[idx]))
		}
	}
	return strings.Join(output, "\n"), nil
}

func shaSum(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return "", err
	}
	resolvedPaths, err := resolveToolPaths(pathsArg)
	if err != nil {
		return "", err
	}
	algorithm, err := extractIntArg(args, "algorithm", 1)
	if err != nil {
		return "", err
	}
	if algorithm != 1 && algorithm != 256 && algorithm != 512 {
		return "", fmt.Errorf("invalid algorithm, only 1, 256, or 512 are valid")
	}

	for _, path := range resolvedPaths {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		if err := ensureFileWithinLimit(path); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		file.Close()
	}

	var cmdArgs []string
	if algorithm != 1 {
		cmdArgs = append(cmdArgs, "-a", fmt.Sprintf("%d", algorithm))
	}
	cmdArgs = append(cmdArgs, resolvedPaths...)

	outputStr, err := runCoreCommand(ctx, coreshasum.New(), cmdArgs)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(outputStr, "\n"), nil
}

func base64Tool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	if err := ensureFileWithinLimit(resolved); err != nil {
		return "", err
	}

	var cmdArgs []string
	if getBoolArg(args, "decode") {
		cmdArgs = append(cmdArgs, "-d")
	}
	cmdArgs = append(cmdArgs, resolved)

	output, err := runCoreCommand(ctx, corebase64.New(), cmdArgs)
	if err != nil {
		return "", err
	}
	if getBoolArg(args, "decode") && !isTextContent([]byte(output)) {
		return "", fmt.Errorf("decoded output appears to be binary")
	}
	return strings.TrimRight(output, "\n"), nil
}

func unameTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	sysname, _ := readProcFirstLine("/proc/sys/kernel/ostype")
	if sysname == "" {
		sysname = runtime.GOOS
	}
	nodename, _ := readProcFirstLine("/proc/sys/kernel/hostname")
	if nodename == "" {
		host, err := os.Hostname()
		if err == nil {
			nodename = host
		}
	}
	release, _ := readProcFirstLine("/proc/sys/kernel/osrelease")
	if release == "" {
		release = "unknown"
	}
	version, _ := readProcFirstLine("/proc/sys/kernel/version")
	if version == "" {
		version = runtime.Version()
	}
	machine := runtime.GOARCH

	output := strings.TrimSpace(strings.Join([]string{sysname, nodename, release, version, machine}, " "))
	if output == "" {
		return "", fmt.Errorf("unable to determine system information")
	}
	return output, nil
}

func hostnameTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	hostname, _ := readProcFirstLine("/proc/sys/kernel/hostname")
	if hostname == "" {
		host, err := os.Hostname()
		if err != nil {
			return "", fmt.Errorf("failed to determine hostname: %v", err)
		}
		hostname = host
	}
	return hostname, nil
}

func uptimeTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	line, err := readProcFirstLine("/proc/uptime")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			seconds, fallbackErr := uptimeFallbackSeconds()
			if fallbackErr != nil {
				return "", fallbackErr
			}
			return formatUptime(seconds), nil
		}
		return "", err
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", fmt.Errorf("unable to parse uptime")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", fmt.Errorf("unable to parse uptime: %v", err)
	}
	return formatUptime(seconds), nil
}

func freeTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	entries, err := readMemInfo()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			entries, err = readMemInfoFallback()
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	keys := []string{"MemTotal", "MemFree", "MemAvailable", "Buffers", "Cached"}
	var output []string
	for _, key := range keys {
		if value, ok := entries[key]; ok {
			output = append(output, fmt.Sprintf("%s: %s", key, value))
		}
	}
	if len(output) == 0 {
		return "", fmt.Errorf("memory information unavailable")
	}
	return strings.Join(output, "\n"), nil
}

func dfTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path := "."
	if args != nil {
		path = getPathArg(args)
	}
	if path == "" {
		path = "."
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}

	size, free, available, err := diskUsage(resolved)
	if err != nil {
		return "", err
	}
	used := size - free
	usePercent := 0.0
	if size > 0 {
		usePercent = (float64(used) / float64(size)) * 100
	}

	lines := []string{
		fmt.Sprintf("Path: %s", resolved),
		fmt.Sprintf("Size: %s (%d bytes)", formatBytes(size), size),
		fmt.Sprintf("Used: %s (%d bytes)", formatBytes(used), used),
		fmt.Sprintf("Available: %s (%d bytes)", formatBytes(available), available),
		fmt.Sprintf("Use%%: %.1f%%", usePercent),
	}
	return strings.Join(lines, "\n"), nil
}

func duTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path := "."
	if args != nil {
		path = getPathArg(args)
	}
	if path == "" {
		path = "."
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}

	limits := getLimits()
	maxDepth := limits.MaxDirectoryDepth
	if depth, err := extractIntArg(args, "max_depth", 0); err != nil {
		return "", err
	} else if depth > 0 && depth < maxDepth {
		maxDepth = depth
	}
	if maxDepth <= 0 {
		maxDepth = 1
	}
	maxEntries := limits.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 2000
	}

	total, err := computeDiskUsage(ctx, resolved, info, maxDepth, maxEntries)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Path: %s\nTotal: %s (%d bytes)", resolved, formatBytes(total), total), nil
}

func psTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	filter, _ := getStringLike(args["name"])
	limit, err := extractIntArg(args, "limit", 200)
	if err != nil {
		return "", err
	}
	if limit <= 0 {
		return "", fmt.Errorf("limit must be positive")
	}

	limits := getLimits()
	if limits.MaxDirectoryEntries > 0 && limit > limits.MaxDirectoryEntries {
		limit = limits.MaxDirectoryEntries
	}

	processes, err := listProcesses(ctx, filter, limit)
	if err != nil {
		return "", err
	}
	if len(processes) == 0 {
		return "No processes found", nil
	}

	var output []string
	output = append(output, "PID COMMAND")
	for _, proc := range processes {
		output = append(output, fmt.Sprintf("%d %s", proc.PID, proc.Command))
	}
	return strings.Join(output, "\n"), nil
}

func pidofTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	name, err := extractStringArg(args, "name")
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", fmt.Errorf("name must be non-empty")
	}

	pids, err := findProcessIDs(ctx, name)
	if err != nil {
		return "", err
	}
	if len(pids) == 0 {
		return "no matching process found", nil
	}
	parts := make([]string, len(pids))
	for i, pid := range pids {
		parts[i] = strconv.Itoa(pid)
	}
	return strings.Join(parts, " "), nil
}

func idTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	userName, _ := getStringLike(args["user"])

	var target *user.User
	var err error
	if userName == "" {
		target, err = user.Current()
	} else {
		target, err = user.Lookup(userName)
	}
	if err != nil {
		return "", err
	}

	uid := target.Uid
	gid := target.Gid
	username := target.Username
	groupName := gid
	if group, err := user.LookupGroupId(gid); err == nil && group.Name != "" {
		groupName = group.Name
	}

	groups, err := target.GroupIds()
	if err != nil {
		groups = nil
	}
	groupParts := make([]string, 0, len(groups))
	for _, groupID := range groups {
		name := groupID
		if grp, err := user.LookupGroupId(groupID); err == nil && grp.Name != "" {
			name = grp.Name
		}
		groupParts = append(groupParts, fmt.Sprintf("%s(%s)", groupID, name))
	}
	if len(groupParts) == 0 {
		groupParts = append(groupParts, fmt.Sprintf("%s(%s)", gid, groupName))
	}

	return fmt.Sprintf("uid=%s(%s) gid=%s(%s) groups=%s", uid, username, gid, groupName, strings.Join(groupParts, ",")), nil
}

func echoTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	var parts []string
	if args != nil {
		if rawParts, ok := args["parts"]; ok {
			parsed, err := extractStringSliceArg(map[string]interface{}{"parts": rawParts}, "parts")
			if err != nil {
				return "", err
			}
			parts = append(parts, parsed...)
		}
		if text, ok := getStringLike(args["text"]); ok {
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " "), nil
}

func seqTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	end, ok, err := getOptionalIntArg(args, "end")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("missing or invalid 'end' parameter")
	}
	start := 1
	if val, ok, err := getOptionalIntArg(args, "start"); err != nil {
		return "", err
	} else if ok {
		start = val
	}
	step := 1
	if val, ok, err := getOptionalIntArg(args, "step"); err != nil {
		return "", err
	} else if ok {
		step = val
	}
	if step == 0 {
		return "", fmt.Errorf("step must be non-zero")
	}
	if start < end && step < 0 {
		return "", fmt.Errorf("step must be positive when start < end")
	}
	if start > end && step > 0 {
		return "", fmt.Errorf("step must be negative when start > end")
	}

	limits := getLimits()
	maxEntries := limits.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 1000
	}

	var output []string
	count := 0
	for i := start; ; i += step {
		if (step > 0 && i > end) || (step < 0 && i < end) {
			break
		}
		output = append(output, strconv.Itoa(i))
		count++
		if count >= maxEntries {
			return "", fmt.Errorf("sequence length exceeds limit of %d", maxEntries)
		}
	}
	return strings.Join(output, "\n"), nil
}

func printenvTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	if args != nil {
		if name, ok := getStringLike(args["name"]); ok {
			value, found := os.LookupEnv(name)
			if !found {
				return "", fmt.Errorf("environment variable not set")
			}
			return value, nil
		}
	}
	env := os.Environ()
	sort.Strings(env)
	return strings.Join(env, "\n"), nil
}

func ttyTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return "not a tty", nil
	}
	return "/dev/tty", nil
}

func whichTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	name, err := extractStringArg(args, "name")
	if err != nil {
		return "", err
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "not found", nil
	}
	return path, nil
}

func mkfifoTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	mode := uint32(0o600)
	if val, ok := getStringLike(args["mode"]); ok {
		if err := validateMode(val); err != nil {
			return "", err
		}
		parsed, err := parseModeString(val)
		if err != nil {
			return "", err
		}
		mode = uint32(parsed)
	}
	if err := mkfifoPath(resolved, mode); err != nil {
		return "", err
	}
	return fmt.Sprintf("Created fifo %s", resolved), nil
}

func mktempTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	baseResolved, err := resolveBaseDir()
	if err != nil {
		return "", err
	}
	tempRoot := filepath.Join(baseResolved, ".tmp")
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return "", err
	}
	prefix := "tmp"
	if args != nil {
		if val, ok := getStringLike(args["prefix"]); ok && strings.TrimSpace(val) != "" {
			prefix = val
		}
	}
	dirRequested := false
	if args != nil {
		dirRequested = getBoolArg(args, "dir")
	}
	if dirRequested {
		path, err := os.MkdirTemp(tempRoot, prefix)
		if err != nil {
			return "", err
		}
		return path, nil
	}
	file, err := os.CreateTemp(tempRoot, prefix)
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func findTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path := "."
	if args != nil {
		path = getPathArg(args)
	}
	if path == "" {
		path = "."
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}

	limits := getLimits()
	maxDepth := limits.MaxDirectoryDepth
	if depth, err := extractIntArg(args, "max_depth", 0); err != nil {
		return "", err
	} else if depth > 0 && depth < maxDepth {
		maxDepth = depth
	}
	if maxDepth <= 0 {
		maxDepth = 1
	}
	maxEntries := limits.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 2000
	}
	showHidden := false
	pattern := ""
	typeFilter := ""
	if args != nil {
		showHidden = getBoolArg(args, "show_hidden")
		pattern, _ = getStringLike(args["name"])
		typeFilter, _ = getStringLike(args["type"])
	}
	typeFilter = strings.ToLower(strings.TrimSpace(typeFilter))
	if typeFilter != "" && typeFilter != "file" && typeFilter != "dir" {
		return "", fmt.Errorf("type must be 'file' or 'dir'")
	}

	entries, err := walkDirEntries(ctx, resolved, walkOptions{
		maxDepth:   maxDepth,
		maxEntries: maxEntries,
		showHidden: showHidden,
		pattern:    pattern,
		typeFilter: typeFilter,
	})
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	matches := make([]string, 0, len(entries))
	for _, entry := range entries {
		matches = append(matches, entry.Path)
	}
	return strings.Join(matches, "\n"), nil
}

func chmodTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	modeStr, err := extractStringArg(args, "mode")
	if err != nil {
		return "", err
	}
	if err := validateMode(modeStr); err != nil {
		return "", err
	}
	mode, err := parseModeString(modeStr)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	if err := ensureOwnedByCurrentUser(resolved); err != nil {
		return "", err
	}
	if err := chmodPath(resolved, mode); err != nil {
		return "", err
	}
	return fmt.Sprintf("Permissions updated for %s", resolved), nil
}

func dateTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	now := time.Now()
	format := ""
	if args != nil {
		format, _ = getStringLike(args["format"])
	}
	switch strings.TrimSpace(strings.ToLower(format)) {
	case "":
		return now.Format(time.RFC3339), nil
	case "unix":
		return strconv.FormatInt(now.Unix(), 10), nil
	default:
		return now.Format(format), nil
	}
}

type processInfo struct {
	PID     int
	Command string
}

func readProcFirstLine(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	return strings.TrimSpace(line), nil
}

func readProcCmdline(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", nil
	}
	cmdline := strings.TrimSpace(strings.ReplaceAll(string(data), "\x00", " "))
	return cmdline, nil
}

func readMemInfo() (map[string]string, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	entries := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value := fields[1]
		if len(fields) > 2 {
			value = value + " " + fields[2]
		}
		entries[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func formatUptime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int64(seconds)
	days := total / int64(24*time.Hour/time.Second)
	remaining := total % int64(24*time.Hour/time.Second)
	hours := remaining / int64(time.Hour/time.Second)
	remaining = remaining % int64(time.Hour/time.Second)
	minutes := remaining / int64(time.Minute/time.Second)
	secs := remaining % int64(time.Minute/time.Second)
	if days > 0 {
		return fmt.Sprintf("up %dd %02dh%02dm%02ds", days, hours, minutes, secs)
	}
	return fmt.Sprintf("up %02dh%02dm%02ds", hours, minutes, secs)
}

func formatBytes(size int64) string {
	if size < 0 {
		size = 0
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	unitIdx := 0
	for unitIdx < len(units)-1 && value >= 1024 {
		value /= 1024
		unitIdx++
	}
	if unitIdx == 0 {
		return fmt.Sprintf("%d %s", size, units[unitIdx])
	}
	return fmt.Sprintf("%.1f %s", value, units[unitIdx])
}

func parseModeString(mode string) (os.FileMode, error) {
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode: %v", err)
	}
	return os.FileMode(parsed), nil
}

func computeDiskUsage(ctx context.Context, root string, info os.FileInfo, maxDepth int, maxEntries int) (int64, error) {
	if !info.IsDir() {
		return info.Size(), nil
	}
	base := root
	total := int64(0)
	entries := 0

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ensureContext(ctx); err != nil {
			return err
		}
		depth := relativeDepth(base, path)
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entries++
		if maxEntries > 0 && entries > maxEntries {
			return fmt.Errorf("directory entry limit exceeded")
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func relativeDepth(base, path string) int {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1
}

func listProcesses(ctx context.Context, filter string, limit int) ([]processInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return listProcessesFallback(ctx, filter, limit)
		}
		return nil, err
	}
	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	sort.Ints(pids)

	filter = strings.ToLower(filter)
	processes := make([]processInfo, 0, limit)
	for _, pid := range pids {
		if err := ensureContext(ctx); err != nil {
			return nil, err
		}
		command, _ := readProcFirstLine(fmt.Sprintf("/proc/%d/comm", pid))
		if command == "" {
			command, _ = readProcCmdline(pid)
		}
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(command), filter) {
			continue
		}
		processes = append(processes, processInfo{PID: pid, Command: command})
		if len(processes) >= limit {
			break
		}
	}
	return processes, nil
}

func findProcessIDs(ctx context.Context, name string) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return findProcessIDsFallback(ctx, name)
		}
		return nil, err
	}
	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	sort.Ints(pids)

	limits := getLimits()
	maxEntries := limits.MaxDirectoryEntries
	if maxEntries <= 0 {
		maxEntries = 2000
	}
	if len(pids) > maxEntries {
		pids = pids[:maxEntries]
	}

	nameLower := strings.ToLower(name)
	var matches []int
	for _, pid := range pids {
		if err := ensureContext(ctx); err != nil {
			return nil, err
		}
		command, _ := readProcFirstLine(fmt.Sprintf("/proc/%d/comm", pid))
		if command == "" {
			command, _ = readProcCmdline(pid)
		}
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		cmdName := strings.ToLower(command)
		if fields := strings.Fields(command); len(fields) > 0 {
			cmdName = strings.ToLower(filepath.Base(fields[0]))
		}
		if strings.EqualFold(command, name) || cmdName == nameLower {
			matches = append(matches, pid)
		}
	}
	return matches, nil
}

func linkPath(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	target, err := extractStringArg(args, "target")
	if err != nil {
		return "", err
	}
	linkPathArg, err := extractStringArg(args, "link_path")
	if err != nil {
		return "", err
	}

	resolvedTarget, err := resolveToolPath(target)
	if err != nil {
		return "", err
	}
	resolvedLink, err := resolveToolPath(linkPathArg)
	if err != nil {
		return "", err
	}

	if getBoolArg(args, "force") {
		if _, err := os.Lstat(resolvedLink); err == nil {
			if err := os.Remove(resolvedLink); err != nil {
				return "", err
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}

	if getBoolArg(args, "symbolic") {
		if err := os.Symlink(resolvedTarget, resolvedLink); err != nil {
			return "", err
		}
		return fmt.Sprintf("Created symlink %s -> %s", resolvedLink, resolvedTarget), nil
	}

	if err := os.Link(resolvedTarget, resolvedLink); err != nil {
		return "", err
	}
	return fmt.Sprintf("Created link %s -> %s", resolvedLink, resolvedTarget), nil
}

func printWorkingDirectory(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}
	return workdir, nil
}

func dirNamePath(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func baseNamePath(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func truncateFile(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	size, err := extractSizeArg(args, "size")
	if err != nil {
		return "", err
	}
	limits := getLimits()
	if limits.MaxFileSizeBytes > 0 && size > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("size exceeds maximum of %d bytes", limits.MaxFileSizeBytes)
	}

	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(resolved); err != nil {
		if os.IsNotExist(err) && !getBoolArg(args, "no_create") {
			file, createErr := os.Create(resolved)
			if createErr != nil {
				return "", createErr
			}
			file.Close()
		} else {
			return "", err
		}
	}

	if err := os.Truncate(resolved, size); err != nil {
		return "", err
	}
	return fmt.Sprintf("Truncated %s to %d bytes", resolved, size), nil
}

func readLinkPath(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	resolved, err := resolveToolPathNoSymlink(path)
	if err != nil {
		return "", err
	}

	linkTarget, err := os.Readlink(resolved)
	if err != nil {
		return "", err
	}

	if getBoolArg(args, "follow") {
		final, err := filepath.EvalSymlinks(resolved)
		if err != nil {
			return "", err
		}
		baseResolved, err := resolveBaseDir()
		if err != nil {
			return "", err
		}
		final, err = ensureResolvedPathWithinBase(final, baseResolved)
		if err != nil {
			return "", err
		}
		return final, nil
	}

	if filepath.IsAbs(linkTarget) {
		baseResolved, err := resolveBaseDir()
		if err != nil {
			return "", err
		}
		if _, err := ensureResolvedPathWithinBase(linkTarget, baseResolved); err != nil {
			return "", err
		}
		return linkTarget, nil
	}

	baseResolved, err := resolveBaseDir()
	if err != nil {
		return "", err
	}
	relTarget := filepath.Clean(filepath.Join(filepath.Dir(resolved), linkTarget))
	if _, err := ensureResolvedPathWithinBase(relTarget, baseResolved); err != nil {
		return "", err
	}
	return linkTarget, nil
}

func realpathPath(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}

	resolved, err := resolveToolPath(path)
	if err != nil {
		return "", err
	}

	final, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", err
	}
	baseResolved, err := resolveBaseDir()
	if err != nil {
		return "", err
	}
	final, err = ensureResolvedPathWithinBase(final, baseResolved)
	if err != nil {
		return "", err
	}
	return final, nil
}

func extractStringArg(args map[string]interface{}, key string) (string, error) {
	if args == nil {
		return "", fmt.Errorf("missing or invalid '%s' parameter", key)
	}
	if val, ok := getStringLike(args[key]); ok {
		return val, nil
	}
	return "", fmt.Errorf("missing or invalid '%s' parameter", key)
}

func extractStringSliceArg(args map[string]interface{}, key string) ([]string, error) {
	if args == nil {
		return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
	val, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
	}

	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
		}
		return []string{v}, nil
	case []string:
		return filterStringSlice(v, key)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			str, ok := item.(string)
			if !ok || strings.TrimSpace(str) == "" {
				return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
			}
			out = append(out, str)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
}

func filterStringSlice(items []string, key string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
	return out, nil
}

func extractPaths(args map[string]interface{}, primary, fallback string) ([]string, error) {
	if args == nil {
		return nil, fmt.Errorf("missing or invalid '%s' parameter", primary)
	}
	if _, ok := args[primary]; ok {
		return extractStringSliceArg(args, primary)
	}
	if fallback != "" {
		if path, ok := getStringLike(args[fallback]); ok {
			return []string{path}, nil
		}
	}
	return nil, fmt.Errorf("missing or invalid '%s' parameter", primary)
}

func resolveToolPaths(paths []string) ([]string, error) {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolvedPath, err := resolveToolPath(path)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedPath)
	}
	return resolved, nil
}

func extractSizeArg(args map[string]interface{}, key string) (int64, error) {
	if args == nil {
		return 0, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
	val, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
	switch v := val.(type) {
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("size must be non-negative")
		}
		return int64(v), nil
	case int64:
		if v < 0 {
			return 0, fmt.Errorf("size must be non-negative")
		}
		return v, nil
	case int:
		if v < 0 {
			return 0, fmt.Errorf("size must be non-negative")
		}
		return int64(v), nil
	default:
		return 0, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
}

func validatePathsArg(primary, fallback string) func(map[string]interface{}) error {
	return func(args map[string]interface{}) error {
		_, err := extractPaths(args, primary, fallback)
		return err
	}
}

func validateRequiredStrings(required []string, listKeys []string) func(map[string]interface{}) error {
	return func(args map[string]interface{}) error {
		for _, key := range required {
			if _, err := extractStringArg(args, key); err != nil {
				return err
			}
		}
		for _, key := range listKeys {
			if _, err := extractStringSliceArg(args, key); err != nil {
				return err
			}
		}
		return nil
	}
}

func validateTruncateArgs(args map[string]interface{}) error {
	if _, err := extractPathArg(args); err != nil {
		return err
	}
	if _, err := extractSizeArg(args, "size"); err != nil {
		return err
	}
	return nil
}

func resolveListPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" || path == "." {
		workdir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine working directory: %v", err)
		}
		return workdir, nil
	}
	return validatePathWithinWorkdir(path)
}

func filterHiddenOutput(output string) string {
	lines := strings.Split(output, "\n")
	kept := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			kept = append(kept, line)
			continue
		}
		if containsHiddenSegment(trimmed) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func containsHiddenSegment(path string) bool {
	cleaned := filepath.Clean(path)
	parts := strings.Split(cleaned, string(os.PathSeparator))
	for _, part := range parts {
		if part == "." || part == ".." || part == "" {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func readHeadTail(ctx context.Context, args map[string]interface{}, head bool, defaultLines int) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	pathsArg, err := extractPaths(args, "paths", "path")
	if err != nil {
		return "", err
	}
	resolvedPaths, err := resolveToolPaths(pathsArg)
	if err != nil {
		return "", err
	}
	linesCount, err := extractIntArg(args, "lines", defaultLines)
	if err != nil {
		return "", err
	}
	if linesCount <= 0 {
		return "", fmt.Errorf("lines must be positive")
	}

	multiFile := len(resolvedPaths) > 1
	var output []string
	for idx, path := range resolvedPaths {
		if err := ensureContext(ctx); err != nil {
			return "", err
		}
		lines, err := readTextLines(path)
		if err != nil {
			return "", err
		}
		if multiFile {
			output = append(output, fmt.Sprintf("==> %s <==", pathsArg[idx]))
		}
		if len(lines) > 0 {
			if head {
				if len(lines) > linesCount {
					lines = lines[:linesCount]
				}
			} else {
				if len(lines) > linesCount {
					lines = lines[len(lines)-linesCount:]
				}
			}
			output = append(output, lines...)
		}
		if multiFile && idx < len(resolvedPaths)-1 {
			output = append(output, "")
		}
	}
	return strings.Join(output, "\n"), nil
}

func readFileLimited(path string, allowBinary bool) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path '%s' is a directory", path)
	}
	limits := getLimits()
	if limits.MaxFileSizeBytes > 0 && info.Size() > limits.MaxFileSizeBytes {
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !allowBinary && !isTextContent(data) {
		return nil, fmt.Errorf("file appears to be binary; tool supports text only")
	}
	return data, nil
}

func ensureFileWithinLimit(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path '%s' is a directory", path)
	}
	limits := getLimits()
	if limits.MaxFileSizeBytes > 0 && info.Size() > limits.MaxFileSizeBytes {
		return fmt.Errorf("file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}
	return nil
}

func readTextLines(path string) ([]string, error) {
	data, err := readFileLimited(path, false)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.Split(text, "\n"), nil
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func countWords(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), len(data)+1)
	scanner.Split(bufio.ScanWords)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}

func extractIntArg(args map[string]interface{}, key string, defaultVal int) (int, error) {
	if args == nil {
		return defaultVal, nil
	}
	val, ok := args[key]
	if !ok {
		return defaultVal, nil
	}
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
}

func getOptionalIntArg(args map[string]interface{}, key string) (int, bool, error) {
	if args == nil {
		return 0, false, nil
	}
	val, ok := args[key]
	if !ok {
		return 0, false, nil
	}
	switch v := val.(type) {
	case float64:
		return int(v), true, nil
	case int:
		return v, true, nil
	case int64:
		return int(v), true, nil
	default:
		return 0, false, fmt.Errorf("missing or invalid '%s' parameter", key)
	}
}

func buildTranslationMap(from, to string) map[rune]rune {
	fromRunes := []rune(from)
	toRunes := []rune(to)
	mapping := make(map[rune]rune, len(fromRunes))
	var last rune
	if len(toRunes) > 0 {
		last = toRunes[len(toRunes)-1]
	}
	for i, r := range fromRunes {
		switch {
		case i < len(toRunes):
			mapping[r] = toRunes[i]
		case len(toRunes) > 0:
			mapping[r] = last
		default:
			mapping[r] = r
		}
	}
	return mapping
}

func validateGrepArgs(args map[string]interface{}) error {
	if _, err := extractStringArg(args, "pattern"); err != nil {
		return err
	}
	if _, err := extractPaths(args, "paths", "path"); err != nil {
		return err
	}
	return nil
}

func validateTranslateArgs(args map[string]interface{}) error {
	if _, err := extractStringArg(args, "from"); err != nil {
		return err
	}
	if _, err := extractStringArg(args, "to"); err != nil {
		return err
	}
	if _, ok := getStringLike(args["path"]); ok {
		return nil
	}
	if _, ok := getStringLike(args["input"]); ok {
		return nil
	}
	return fmt.Errorf("missing or invalid 'path' or 'input' parameter")
}

func validateTeeArgs(args map[string]interface{}) error {
	if _, err := extractStringArg(args, "content"); err != nil {
		return err
	}
	if _, err := extractPaths(args, "paths", "path"); err != nil {
		return err
	}
	return nil
}

func validateCommArgs(args map[string]interface{}) error {
	if _, err := extractStringArg(args, "path1"); err != nil {
		return err
	}
	if _, err := extractStringArg(args, "path2"); err != nil {
		return err
	}
	return nil
}

func validateMode(mode string) error {
	if strings.TrimSpace(mode) == "" {
		return fmt.Errorf("mode cannot be empty")
	}
	if len(mode) > 4 {
		return fmt.Errorf("mode must be an octal string like 755 or 0755")
	}
	for _, r := range mode {
		if r < '0' || r > '7' {
			return fmt.Errorf("mode must be an octal string like 755 or 0755")
		}
	}
	return nil
}

func validateSeqArgs(args map[string]interface{}) error {
	if _, ok, err := getOptionalIntArg(args, "end"); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("missing or invalid 'end' parameter")
	}
	if _, ok, err := getOptionalIntArg(args, "start"); err != nil {
		return err
	} else if ok {
		return nil
	}
	if _, ok, err := getOptionalIntArg(args, "step"); err != nil {
		return err
	} else if ok {
		return nil
	}
	return nil
}

func validateChmodArgs(args map[string]interface{}) error {
	if _, err := extractPathArg(args); err != nil {
		return err
	}
	mode, err := extractStringArg(args, "mode")
	if err != nil {
		return err
	}
	return validateMode(mode)
}
