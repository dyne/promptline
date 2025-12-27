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

type lsArgs struct {
	Path       string `json:"path,omitempty" jsonschema:"description=Directory path to list (default: current directory)"`
	Recursive  bool   `json:"recursive,omitempty" jsonschema:"description=List directories recursively"`
	ShowHidden bool   `json:"show_hidden,omitempty" jsonschema:"description=Include hidden files"`
}

type catArgs struct {
	Paths []string `json:"paths,omitempty" jsonschema:"description=File paths to concatenate"`
	Path  string   `json:"path,omitempty" jsonschema:"description=Single file path to concatenate"`
}

type headArgs struct {
	Paths []string `json:"paths,omitempty" jsonschema:"description=File paths to read"`
	Path  string   `json:"path,omitempty" jsonschema:"description=Single file path to read"`
	Lines float64  `json:"lines,omitempty" jsonschema:"description=Number of lines to return (default: 10)"`
}

type tailArgs struct {
	Paths []string `json:"paths,omitempty" jsonschema:"description=File paths to read"`
	Path  string   `json:"path,omitempty" jsonschema:"description=Single file path to read"`
	Lines float64  `json:"lines,omitempty" jsonschema:"description=Number of lines to return (default: 10)"`
}

type wcArgs struct {
	Paths []string `json:"paths,omitempty" jsonschema:"description=File paths to count"`
	Path  string   `json:"path,omitempty" jsonschema:"description=Single file path to count"`
}

type md5sumArgs struct {
	Paths []string `json:"paths,omitempty" jsonschema:"description=File paths to hash"`
	Path  string   `json:"path,omitempty" jsonschema:"description=Single file path to hash"`
}

type shasumArgs struct {
	Paths     []string `json:"paths,omitempty" jsonschema:"description=File paths to hash"`
	Path      string   `json:"path,omitempty" jsonschema:"description=Single file path to hash"`
	Algorithm float64  `json:"algorithm,omitempty" jsonschema:"description=SHA algorithm (1, 256, or 512)"`
}

type echoArgs struct {
	Text  string   `json:"text,omitempty" jsonschema:"description=Text to echo"`
	Parts []string `json:"parts,omitempty" jsonschema:"description=Text parts to join with spaces"`
}

type linkArgs struct {
	Target   string `json:"target" jsonschema:"description=Existing target path"`
	LinkPath string `json:"link_path" jsonschema:"description=New link path"`
	Symbolic bool   `json:"symbolic,omitempty" jsonschema:"description=Create a symbolic link instead of hard link"`
	Force    bool   `json:"force,omitempty" jsonschema:"description=Remove existing destination before linking"`
}

type translateArgs struct {
	From  string `json:"from" jsonschema:"description=Characters to replace"`
	To    string `json:"to" jsonschema:"description=Replacement characters"`
	Path  string `json:"path,omitempty" jsonschema:"description=File path to process"`
	Input string `json:"input,omitempty" jsonschema:"description=Inline text to process (if path not provided)"`
}

type truncateArgs struct {
	Path     string  `json:"path" jsonschema:"description=File path to truncate"`
	Size     float64 `json:"size" jsonschema:"description=Size in bytes"`
	NoCreate bool    `json:"no_create,omitempty" jsonschema:"description=Do not create file if missing"`
}

type readlinkArgs struct {
	Path   string `json:"path" jsonschema:"description=Symlink path to read"`
	Follow bool   `json:"follow,omitempty" jsonschema:"description=Resolve symlinks recursively"`
}

type sortArgs struct {
	Path    string `json:"path" jsonschema:"description=File path to sort"`
	Reverse bool   `json:"reverse,omitempty" jsonschema:"description=Reverse sort order"`
}

type commArgs struct {
	Path1 string `json:"path1" jsonschema:"description=First file path"`
	Path2 string `json:"path2" jsonschema:"description=Second file path"`
}

type stringsArgs struct {
	Path      string  `json:"path" jsonschema:"description=File path to scan"`
	MinLength float64 `json:"min_length,omitempty" jsonschema:"description=Minimum string length (default: 4)"`
}

type moreArgs struct {
	Path  string  `json:"path" jsonschema:"description=File path to read"`
	Lines float64 `json:"lines,omitempty" jsonschema:"description=Number of lines to return (default: 40)"`
}

type hexdumpArgs struct {
	Path     string  `json:"path" jsonschema:"description=File path to dump"`
	MaxBytes float64 `json:"max_bytes,omitempty" jsonschema:"description=Maximum bytes to display (default: 512)"`
}

type cmpArgs struct {
	Path1 string `json:"path1" jsonschema:"description=First file path"`
	Path2 string `json:"path2" jsonschema:"description=Second file path"`
}

type base64Args struct {
	Path   string `json:"path" jsonschema:"description=File path to encode or decode"`
	Decode bool   `json:"decode,omitempty" jsonschema:"description=Decode base64 input"`
}

type seqArgs struct {
	Start float64 `json:"start,omitempty" jsonschema:"description=Start value (default: 1 if end specified)"`
	End   float64 `json:"end,omitempty" jsonschema:"description=End value"`
	Step  float64 `json:"step,omitempty" jsonschema:"description=Step value (default: 1)"`
}

type mkfifoArgs struct {
	Path string `json:"path" jsonschema:"description=FIFO path to create"`
	Mode string `json:"mode,omitempty" jsonschema:"description=Octal permission mode (default: 600)"`
}

type findArgs struct {
	Path       string  `json:"path,omitempty" jsonschema:"description=Root path to search (default: current directory)"`
	Name       string  `json:"name,omitempty" jsonschema:"description=Glob pattern to match file names"`
	Type       string  `json:"type,omitempty" jsonschema:"description=Filter by type: file or dir"`
	MaxDepth   float64 `json:"max_depth,omitempty" jsonschema:"description=Maximum depth to traverse"`
	ShowHidden bool    `json:"show_hidden,omitempty" jsonschema:"description=Include hidden entries"`
}

type chmodArgs struct {
	Path string `json:"path" jsonschema:"description=Path to modify"`
	Mode string `json:"mode" jsonschema:"description=Octal permission mode (e.g., 644)"`
}

type noArgs struct{}

type realpathArgs struct {
	Path string `json:"path" jsonschema:"description=Path to resolve"`
}

type uniqArgs struct {
	Path string `json:"path" jsonschema:"description=File path to process"`
}

type pathArg struct {
	Path string `json:"path" jsonschema:"description=Path to evaluate"`
}

type dfArgs struct {
	Path string `json:"path,omitempty" jsonschema:"description=Path to inspect (default: current directory)"`
}

type duArgs struct {
	Path     string  `json:"path,omitempty" jsonschema:"description=Path to inspect (default: current directory)"`
	MaxDepth float64 `json:"max_depth,omitempty" jsonschema:"description=Maximum depth to traverse"`
}

type psArgs struct {
	Name  string  `json:"name,omitempty" jsonschema:"description=Filter processes by substring match"`
	Limit float64 `json:"limit,omitempty" jsonschema:"description=Maximum number of processes to return"`
}

type pidofArgs struct {
	Name string `json:"name" jsonschema:"description=Process name to match"`
}

type idArgs struct {
	User string `json:"user,omitempty" jsonschema:"description=User name to inspect (default: current user)"`
}

type printenvArgs struct {
	Name string `json:"name,omitempty" jsonschema:"description=Specific environment variable to print"`
}

type whichArgs struct {
	Name string `json:"name" jsonschema:"description=Command name to locate"`
}

type mktempArgs struct {
	Dir    bool   `json:"dir,omitempty" jsonschema:"description=Create a temporary directory instead of a file"`
	Prefix string `json:"prefix,omitempty" jsonschema:"description=Prefix for the temporary name"`
}

type dateArgs struct {
	Format string `json:"format,omitempty" jsonschema:"description=Go time layout or 'unix' (default: RFC3339)"`
}
