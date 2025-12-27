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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	searchMarker       = "<<<<<<< SEARCH"
	separatorMarker    = "======="
	replaceMarker      = ">>>>>>> REPLACE"
	maxEditBlocks      = 64
	maxEditBlockBytes  = 64 * 1024
	fuzzyMinSimilarity = 0.8
	fuzzyTieEpsilon    = 0.02
)

type createFileArgs struct {
	Path      string `json:"path" jsonschema:"description=Path to the file to create,minLength=1"`
	Content   string `json:"content" jsonschema:"description=Text content to write,minLength=1"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"description=Overwrite the file if it already exists"`
}

type editFileArgs struct {
	Path  string `json:"path" jsonschema:"description=Path to the file to edit,minLength=1"`
	Edits string `json:"edits" jsonschema:"description=SEARCH/REPLACE blocks to apply,minLength=1"`
}

type searchReplaceEdit struct {
	Search  string
	Replace string
}

type editMatch struct {
	Mode string
}

func validateCreateFileArgs(args map[string]interface{}) error {
	if _, err := extractPathArg(args); err != nil {
		return err
	}
	content, ok := args["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return fmt.Errorf("missing or invalid 'content' parameter")
	}
	if overwrite, ok := args["overwrite"]; ok {
		if _, ok := overwrite.(bool); !ok {
			return fmt.Errorf("missing or invalid 'overwrite' parameter")
		}
	}
	return nil
}

func validateEditFileArgs(args map[string]interface{}) error {
	if _, err := extractPathArg(args); err != nil {
		return err
	}
	edits, ok := args["edits"].(string)
	if !ok || strings.TrimSpace(edits) == "" {
		return fmt.Errorf("missing or invalid 'edits' parameter")
	}
	return nil
}

func createFile(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	content, ok := args["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("missing or invalid 'content' parameter")
	}

	overwrite := false
	if rawOverwrite, ok := args["overwrite"]; ok {
		val, ok := rawOverwrite.(bool)
		if !ok {
			return "", fmt.Errorf("missing or invalid 'overwrite' parameter")
		}
		overwrite = val
	}

	limits := getLimits()
	if int64(len(content)) > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("content exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}
	if !isTextContent([]byte(content)) {
		return "", fmt.Errorf("content appears to be binary; create_file supports text only")
	}

	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}

	resolved, err := resolvePathWithinBaseAllowMissing(path, workdir)
	if err != nil {
		return "", err
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(resolved); err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("path '%s' is a directory", resolved)
		}
		if !overwrite {
			return "", fmt.Errorf("file already exists; set overwrite to true to replace it")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to stat file: %v", err)
	}

	parent := filepath.Dir(resolved)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("failed to create parent directories: %v", err)
	}

	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	if err := os.WriteFile(resolved, []byte(content), mode); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), resolved), nil
}

func editFile(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}
	editsRaw, ok := args["edits"].(string)
	if !ok || strings.TrimSpace(editsRaw) == "" {
		return "", fmt.Errorf("missing or invalid 'edits' parameter")
	}

	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}

	resolved, err := resolvePathWithinBase(path, workdir)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path '%s' is a directory", resolved)
	}

	limits := getLimits()
	if info.Size() > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}

	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	originalBytes, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	if !isTextContent(originalBytes) {
		return "", fmt.Errorf("file appears to be binary; edit_file supports text only")
	}

	edits, err := parseSearchReplaceEdits(editsRaw)
	if err != nil {
		return "", err
	}

	updated, matches, err := applySearchReplaceEdits(string(originalBytes), edits)
	if err != nil {
		return "", err
	}

	if updated == string(originalBytes) {
		return fmt.Sprintf("No changes applied to %s", resolved), nil
	}

	if int64(len(updated)) > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("updated file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}

	if err := os.WriteFile(resolved, []byte(updated), info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Applied %d edits to %s (%s)", len(matches), resolved, summarizeEditMatches(matches)), nil
}

func parseSearchReplaceEdits(input string) ([]searchReplaceEdit, error) {
	lines := strings.Split(input, "\n")
	var edits []searchReplaceEdit

	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		if !isSearchMarkerLine(lines[i]) {
			return nil, fmt.Errorf("unexpected content outside SEARCH/REPLACE blocks")
		}
		i++
		searchStart := i
		for i < len(lines) && strings.TrimSpace(lines[i]) != separatorMarker {
			i++
		}
		if i >= len(lines) {
			return nil, fmt.Errorf("missing %s marker", separatorMarker)
		}
		searchLines := lines[searchStart:i]
		i++ // skip separator
		replaceStart := i
		for i < len(lines) && !isReplaceMarkerLine(lines[i]) {
			i++
		}
		if i >= len(lines) {
			return nil, fmt.Errorf("missing %s marker", replaceMarker)
		}
		replaceLines := lines[replaceStart:i]
		i++ // skip replace marker

		if len(edits) >= maxEditBlocks {
			return nil, fmt.Errorf("too many edit blocks (max %d)", maxEditBlocks)
		}

		searchText := strings.Join(searchLines, "\n")
		replaceText := strings.Join(replaceLines, "\n")
		if strings.TrimSpace(searchText) == "" {
			return nil, fmt.Errorf("search block cannot be empty")
		}
		if len(searchText) > maxEditBlockBytes || len(replaceText) > maxEditBlockBytes {
			return nil, fmt.Errorf("edit block exceeds maximum size of %d bytes", maxEditBlockBytes)
		}

		edits = append(edits, searchReplaceEdit{
			Search:  searchText,
			Replace: replaceText,
		})
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("no SEARCH/REPLACE blocks found")
	}
	return edits, nil
}

func isSearchMarkerLine(line string) bool {
	return matchesMarker(line, '<', "SEARCH")
}

func isReplaceMarkerLine(line string) bool {
	return matchesMarker(line, '>', "REPLACE")
}

func matchesMarker(line string, marker byte, keyword string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	count := 0
	for count < len(trimmed) && trimmed[count] == marker {
		count++
	}
	if count < 6 || count > 8 {
		return false
	}
	if count == len(trimmed) {
		return false
	}
	if !isInlineWhitespace(trimmed[count]) {
		return false
	}
	rest := strings.TrimSpace(trimmed[count:])
	return rest == keyword
}

func isInlineWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t'
}

func applySearchReplaceEdits(content string, edits []searchReplaceEdit) (string, []editMatch, error) {
	updated := content
	matches := make([]editMatch, 0, len(edits))
	for idx, edit := range edits {
		next, match, err := applySingleEdit(updated, edit, idx+1)
		if err != nil {
			return "", nil, err
		}
		updated = next
		matches = append(matches, match)
	}
	return updated, matches, nil
}

func applySingleEdit(content string, edit searchReplaceEdit, index int) (string, editMatch, error) {
	start, end, err := findExactMatch(content, edit.Search)
	if err != nil {
		return "", editMatch{}, fmt.Errorf("edit %d: %v", index, err)
	}
	if start >= 0 {
		updated := replaceSpan(content, start, end, edit.Replace)
		return updated, editMatch{Mode: "exact"}, nil
	}

	start, end, err = findWhitespaceInsensitiveMatch(content, edit.Search)
	if err != nil {
		return "", editMatch{}, fmt.Errorf("edit %d: %v", index, err)
	}
	if start >= 0 {
		matched := content[start:end]
		replacement := reindentReplacement(edit.Replace, indentOfFirstNonEmptyLine(matched))
		replacement = applyLineEndings(matched, replacement)
		updated := replaceSpan(content, start, end, replacement)
		return updated, editMatch{Mode: "whitespace"}, nil
	}

	start, end, err = findFuzzyMatch(content, edit.Search)
	if err != nil {
		return "", editMatch{}, fmt.Errorf("edit %d: %v", index, err)
	}
	if start >= 0 {
		matched := content[start:end]
		replacement := reindentReplacement(edit.Replace, indentOfFirstNonEmptyLine(matched))
		replacement = applyLineEndings(matched, replacement)
		updated := replaceSpan(content, start, end, replacement)
		return updated, editMatch{Mode: "fuzzy"}, nil
	}

	return "", editMatch{}, fmt.Errorf("edit %d: search block not found", index)
}

func findExactMatch(content, search string) (int, int, error) {
	idx := strings.Index(content, search)
	if idx == -1 {
		return -1, -1, nil
	}
	next := strings.Index(content[idx+len(search):], search)
	if next != -1 {
		return -1, -1, fmt.Errorf("search block matches multiple locations")
	}
	return idx, idx + len(search), nil
}

func findWhitespaceInsensitiveMatch(content, search string) (int, int, error) {
	contentLines, offsets := splitLinesWithOffsets(content)
	searchLines := splitSearchLines(search)
	if len(searchLines) == 0 {
		return -1, -1, nil
	}

	matches := 0
	matchIndex := -1
	windowSize := len(searchLines)
	for i := 0; i+windowSize <= len(contentLines); i++ {
		if linesMatchWhitespace(contentLines[i:i+windowSize], searchLines) {
			matches++
			matchIndex = i
		}
	}

	if matches == 0 {
		return -1, -1, nil
	}
	if matches > 1 {
		return -1, -1, fmt.Errorf("search block matches multiple locations")
	}

	start := offsets[matchIndex]
	end := len(content)
	if matchIndex+windowSize < len(offsets) {
		end = offsets[matchIndex+windowSize]
	}
	return start, end, nil
}

func findFuzzyMatch(content, search string) (int, int, error) {
	contentLines, offsets := splitLinesWithOffsets(content)
	searchLines := splitSearchLines(search)
	if len(searchLines) == 0 {
		return -1, -1, nil
	}

	normalizedSearch := normalizeBlock(searchLines)
	if len(normalizedSearch) == 0 {
		return -1, -1, nil
	}

	normalizedContent := normalizeContentLines(contentLines)
	windowSize := len(searchLines)
	bestScore := -1.0
	bestIndex := -1
	tied := false

	for i := 0; i+windowSize <= len(contentLines); i++ {
		window := normalizeBlock(normalizedContent[i : i+windowSize])
		score := similarityScore(normalizedSearch, window)
		if score > bestScore+fuzzyTieEpsilon {
			bestScore = score
			bestIndex = i
			tied = false
			continue
		}
		if bestScore >= 0 && score >= bestScore-fuzzyTieEpsilon {
			tied = true
		}
	}

	if bestScore < fuzzyMinSimilarity {
		return -1, -1, nil
	}
	if tied {
		return -1, -1, fmt.Errorf("search block matches multiple fuzzy locations")
	}

	start := offsets[bestIndex]
	end := len(content)
	if bestIndex+windowSize < len(offsets) {
		end = offsets[bestIndex+windowSize]
	}
	return start, end, nil
}

func splitSearchLines(search string) []string {
	return strings.Split(normalizeToLF(search), "\n")
}

func splitLinesWithOffsets(text string) ([]string, []int) {
	offsets := []int{0}
	for idx, r := range text {
		if r == '\n' {
			offsets = append(offsets, idx+1)
		}
	}
	lines := make([]string, len(offsets))
	for i := range offsets {
		start := offsets[i]
		end := len(text)
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		lines[i] = text[start:end]
	}
	return lines, offsets
}

func normalizeToLF(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func normalizeContentLines(lines []string) []string {
	normalized := make([]string, len(lines))
	for i, line := range lines {
		normalized[i] = normalizeLine(line)
	}
	return normalized
}

func normalizeLine(line string) string {
	trimmed := strings.TrimSuffix(line, "\n")
	trimmed = strings.TrimSuffix(trimmed, "\r")
	if strings.TrimSpace(trimmed) == "" {
		return ""
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

func normalizeBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	normalized := make([]string, len(lines))
	for i, line := range lines {
		if line == "" {
			normalized[i] = ""
			continue
		}
		normalized[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(normalized, "\n")
}

func linesMatchWhitespace(contentLines []string, searchLines []string) bool {
	if len(contentLines) != len(searchLines) {
		return false
	}
	for i := range contentLines {
		if normalizeLine(contentLines[i]) != normalizeLine(searchLines[i]) {
			return false
		}
	}
	return true
}

func similarityScore(a, b string) float64 {
	maxLen := len([]rune(a))
	if len([]rune(b)) > maxLen {
		maxLen = len([]rune(b))
	}
	if maxLen == 0 {
		return 1
	}
	dist := levenshteinDistance(a, b)
	return 1 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}

	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}
	for i := 0; i < len(ar); i++ {
		curr[0] = i + 1
		for j := 0; j < len(br); j++ {
			cost := 0
			if ar[i] != br[j] {
				cost = 1
			}
			insertCost := curr[j] + 1
			deleteCost := prev[j+1] + 1
			replaceCost := prev[j] + cost
			curr[j+1] = minInt(insertCost, deleteCost, replaceCost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func minInt(vals ...int) int {
	min := vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func indentOfFirstNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		return leadingWhitespace(line)
	}
	return ""
}

func leadingWhitespace(line string) string {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return line[:i]
		}
	}
	return line
}

func reindentReplacement(replacement, targetIndent string) string {
	lines := strings.Split(replacement, "\n")
	commonIndent := commonLeadingIndent(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, commonIndent) {
			line = strings.TrimPrefix(line, commonIndent)
		}
		lines[i] = targetIndent + line
	}
	return strings.Join(lines, "\n")
}

func commonLeadingIndent(lines []string) string {
	var indent string
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineIndent := leadingWhitespace(line)
		if indent == "" || len(lineIndent) < len(indent) {
			indent = lineIndent
		}
	}
	return indent
}

func applyLineEndings(matched, replacement string) string {
	if strings.Contains(matched, "\r\n") {
		replacement = normalizeToLF(replacement)
		return strings.ReplaceAll(replacement, "\n", "\r\n")
	}
	return replacement
}

func replaceSpan(content string, start, end int, replacement string) string {
	return content[:start] + replacement + content[end:]
}

func summarizeEditMatches(matches []editMatch) string {
	counts := map[string]int{
		"exact":      0,
		"whitespace": 0,
		"fuzzy":      0,
	}
	for _, match := range matches {
		if _, ok := counts[match.Mode]; ok {
			counts[match.Mode]++
		}
	}
	return fmt.Sprintf("exact=%d whitespace=%d fuzzy=%d", counts["exact"], counts["whitespace"], counts["fuzzy"])
}
