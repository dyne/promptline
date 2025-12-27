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

type copyArgs struct {
	Sources          []string `json:"sources" jsonschema:"description=Source file or directory paths"`
	Destination      string   `json:"destination" jsonschema:"description=Destination path"`
	Recursive        bool     `json:"recursive,omitempty" jsonschema:"description=Copy directories recursively"`
	Force            bool     `json:"force,omitempty" jsonschema:"description=Overwrite existing files"`
	NoFollowSymlinks bool     `json:"no_follow_symlinks,omitempty" jsonschema:"description=Copy symlink itself instead of target"`
}

type moveArgs struct {
	Sources     []string `json:"sources" jsonschema:"description=Source file or directory paths"`
	Destination string   `json:"destination" jsonschema:"description=Destination path"`
	Update      bool     `json:"update,omitempty" jsonschema:"description=Move only when source is newer or destination is missing"`
	NoClobber   bool     `json:"no_clobber,omitempty" jsonschema:"description=Do not overwrite existing files"`
}

type removeArgs struct {
	Paths     []string `json:"paths,omitempty" jsonschema:"description=Paths to remove"`
	Path      string   `json:"path,omitempty" jsonschema:"description=Single path to remove"`
	Recursive bool     `json:"recursive,omitempty" jsonschema:"description=Remove directories recursively"`
	Force     bool     `json:"force,omitempty" jsonschema:"description=Ignore nonexistent files"`
}

type touchArgs struct {
	Paths        []string `json:"paths,omitempty" jsonschema:"description=File paths to touch"`
	Path         string   `json:"path,omitempty" jsonschema:"description=Single file path to touch"`
	Access       bool     `json:"access,omitempty" jsonschema:"description=Change access time only"`
	Modification bool     `json:"modification,omitempty" jsonschema:"description=Change modification time only"`
	NoCreate     bool     `json:"no_create,omitempty" jsonschema:"description=Do not create files if they do not exist"`
	Datetime     string   `json:"datetime,omitempty" jsonschema:"description=RFC3339 timestamp to apply"`
}

type grepArgs struct {
	Pattern    string   `json:"pattern" jsonschema:"description=Pattern to search for (regular expression)"`
	Paths      []string `json:"paths,omitempty" jsonschema:"description=File paths to search"`
	Path       string   `json:"path,omitempty" jsonschema:"description=Single file path to search"`
	IgnoreCase bool     `json:"ignore_case,omitempty" jsonschema:"description=Case-insensitive matching"`
	Recursive  bool     `json:"recursive,omitempty" jsonschema:"description=Search directories recursively"`
	ShowHidden bool     `json:"show_hidden,omitempty" jsonschema:"description=Include hidden files when searching directories"`
	Invert     bool     `json:"invert,omitempty" jsonschema:"description=Select non-matching lines"`
	MaxMatches float64  `json:"max_matches,omitempty" jsonschema:"description=Maximum number of matches to return"`
}

type teeArgs struct {
	Content string   `json:"content" jsonschema:"description=Content to write"`
	Paths   []string `json:"paths,omitempty" jsonschema:"description=File paths to write"`
	Path    string   `json:"path,omitempty" jsonschema:"description=Single file path to write"`
}

type mkdirArgs struct {
	Paths   []string `json:"paths,omitempty" jsonschema:"description=Directory paths to create"`
	Path    string   `json:"path,omitempty" jsonschema:"description=Single directory path to create"`
	Parents bool     `json:"parents,omitempty" jsonschema:"description=Create parent directories as needed"`
	Mode    string   `json:"mode,omitempty" jsonschema:"description=Octal permission mode (e.g., 755)"`
}
