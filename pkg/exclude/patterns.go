package exclude

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Pattern represents a single exclusion pattern
type Pattern struct {
	Raw       string         // Original pattern string
	Regex     *regexp.Regexp // Compiled regex pattern
	Negated   bool           // Pattern starts with !
	DirOnly   bool           // Pattern ends with /
	Recursive bool           // Pattern contains **
}

// PatternSet represents a collection of exclusion patterns
type PatternSet struct {
	patterns []*Pattern
	sources  []string // Track source files for patterns
}

// NewPatternSet creates a new empty pattern set
func NewPatternSet() *PatternSet {
	return &PatternSet{
		patterns: make([]*Pattern, 0),
		sources:  make([]string, 0),
	}
}

// ParsePatterns reads patterns from a file
func ParsePatterns(filePath string) (*PatternSet, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open pattern file %s: %w", filePath, err)
	}
	defer file.Close()

	return ParsePatternsFromReader(file, filePath)
}

// ParsePatternsFromReader reads patterns from any io.Reader
func ParsePatternsFromReader(reader io.Reader, source string) (*PatternSet, error) {
	set := NewPatternSet()
	set.sources = append(set.sources, source)

	scanner := bufio.NewScanner(reader)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pattern, err := parsePattern(line, lineNum)
		if err != nil {
			return nil, fmt.Errorf("error in %s at line %d: %w", source, lineNum, err)
		}

		if pattern != nil {
			set.patterns = append(set.patterns, pattern)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", source, err)
	}

	return set, nil
}

// parsePattern converts a gitignore pattern to a Pattern struct
func parsePattern(line string, lineNum int) (*Pattern, error) {
	pattern := &Pattern{Raw: line}

	// Handle negation
	if strings.HasPrefix(line, "!") {
		pattern.Negated = true
		line = line[1:]
	}

	// Handle directory-only patterns
	if strings.HasSuffix(line, "/") {
		pattern.DirOnly = true
		line = line[:len(line)-1]
	}

	// Check for recursive patterns
	pattern.Recursive = strings.Contains(line, "**")

	// Convert to regex
	regex, err := patternToRegex(line, pattern.DirOnly, pattern.Recursive)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern '%s': %w", line, err)
	}

	pattern.Regex = regex
	return pattern, nil
}

// patternToRegex converts a gitignore pattern to a regex
func patternToRegex(pattern string, dirOnly, recursive bool) (*regexp.Regexp, error) {
	var buf bytes.Buffer
	isAbsolute := strings.HasPrefix(pattern, "/")

	// Remove leading slash for regex matching but remember it was absolute
	if isAbsolute {
		pattern = pattern[1:]
	}

	// Convert pattern components
	for i := 0; i < len(pattern); i++ {
		char := rune(pattern[i])

		switch char {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// Handle ** (recursive wildcard)
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					// **/ matches any number of directories
					buf.WriteString("(?:.*/)?")
					i += 2
				} else {
					// ** matches anything
					buf.WriteString(".*")
					i += 1
				}
			} else {
				// * matches any characters except /
				buf.WriteString("[^/]*")
			}

		case '?':
			// ? matches any single character except /
			buf.WriteString("[^/]")

		case '.':
			// Escape literal dot
			buf.WriteString("\\.")

		case '\\':
			// Handle escaped characters
			if i+1 < len(pattern) {
				buf.WriteString(regexp.QuoteMeta(string(pattern[i+1])))
				i += 1
			}

		default:
			// Regular character or special regex character
			if isSpecialRegexChar(char) {
				buf.WriteString("\\")
			}
			buf.WriteRune(char)
		}
	}

	// Directory-only patterns should match directories
	if dirOnly {
		buf.WriteString("/?$") // Match optional trailing slash
	} else if recursive {
		// Recursive patterns can match files or directories
		buf.WriteString("(?:/|$)")
	} else {
		// Non-recursive, non-directory patterns should only match files
		buf.WriteString("(?:/|$)")
	}

	// For non-absolute patterns, also allow matching after any path component
	if !isAbsolute {
		fullPattern := "(?:^|/)" + buf.String()
		regex, err := regexp.Compile(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex: %w", err)
		}
		return regex, nil
	}

	// Anchor to start of string for absolute patterns
	fullPattern := "^" + buf.String()

	regex, err := regexp.Compile(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	return regex, nil
}

// isSpecialRegexChar checks if a character has special meaning in regex
func isSpecialRegexChar(char rune) bool {
	switch char {
	case '^', '$', '.', '|', '(', ')', '[', ']', '{', '}', '+', '?', '*', '\\':
		return true
	default:
		return false
	}
}

// LoadDefaultPatterns loads default patterns for Nextcloud sync
func LoadDefaultPatterns() *PatternSet {
	patterns := []string{
		".DS_Store",
		"Thumbs.db",
		"*.tmp",
		"*.temp",
		"*.log",
		".git/",
		".svn/",
		"node_modules/",
		".nextcloud-sync/",
		"*.swp",
		"*.swo",
		"*~",
	}

	set := NewPatternSet()
	set.sources = append(set.sources, "default")

	for _, pattern := range patterns {
		parsed, err := parsePattern(pattern, 0)
		if err != nil {
			// Default patterns should never have errors
			continue
		}
		set.patterns = append(set.patterns, parsed)
	}

	return set
}

// AddPattern adds a single pattern to the set
func (ps *PatternSet) AddPattern(pattern string) error {
	parsed, err := parsePattern(pattern, 0)
	if err != nil {
		return fmt.Errorf("invalid pattern '%s': %w", pattern, err)
	}

	ps.patterns = append(ps.patterns, parsed)
	ps.sources = append(ps.sources, "manual")
	return nil
}

// Merge merges another PatternSet into this one
func (ps *PatternSet) Merge(other *PatternSet) {
	if other == nil {
		return
	}

	ps.patterns = append(ps.patterns, other.patterns...)
	ps.sources = append(ps.sources, other.sources...)
}

// GetPatterns returns all patterns in the set
func (ps *PatternSet) GetPatterns() []*Pattern {
	return ps.patterns
}

// GetSources returns the source files for patterns
func (ps *PatternSet) GetSources() []string {
	return ps.sources
}

// Size returns the number of patterns
func (ps *PatternSet) Size() int {
	return len(ps.patterns)
}

// IsEmpty returns true if the pattern set is empty
func (ps *PatternSet) IsEmpty() bool {
	return len(ps.patterns) == 0
}

// LoadFromFile loads patterns from .nextcloudignore file in a directory
func LoadFromFile(dirPath string) (*PatternSet, error) {
	ignoreFile := filepath.Join(dirPath, ".nextcloudignore")

	// Check if ignore file exists
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		// Return empty pattern set if no ignore file
		return NewPatternSet(), nil
	}

	return ParsePatterns(ignoreFile)
}

// LoadFromFiles loads patterns from multiple files
func LoadFromFiles(filePaths []string) (*PatternSet, error) {
	set := NewPatternSet()

	for _, filePath := range filePaths {
		patterns, err := ParsePatterns(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load patterns from %s: %w", filePath, err)
		}
		set.Merge(patterns)
	}

	return set, nil
}
