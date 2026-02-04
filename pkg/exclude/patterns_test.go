package exclude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expected    *Pattern
		expectError bool
	}{
		{
			name:    "simple pattern",
			pattern: "*.txt",
			expected: &Pattern{
				Raw:       "*.txt",
				Negated:   false,
				DirOnly:   false,
				Recursive: false,
			},
		},
		{
			name:    "negated pattern",
			pattern: "!important.txt",
			expected: &Pattern{
				Raw:       "!important.txt",
				Negated:   true,
				DirOnly:   false,
				Recursive: false,
			},
		},
		{
			name:    "directory only pattern",
			pattern: "node_modules/",
			expected: &Pattern{
				Raw:       "node_modules/",
				Negated:   false,
				DirOnly:   true,
				Recursive: false,
			},
		},
		{
			name:    "recursive pattern",
			pattern: "**/cache",
			expected: &Pattern{
				Raw:       "**/cache",
				Negated:   false,
				DirOnly:   false,
				Recursive: true,
			},
		},
		{
			name:    "complex recursive pattern",
			pattern: "src/**/*.go",
			expected: &Pattern{
				Raw:       "src/**/*.go",
				Negated:   false,
				DirOnly:   false,
				Recursive: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePattern(tt.pattern, 1)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)

				// Test pattern fields (regex is complex, just test it's not nil)
				assert.Equal(t, tt.expected.Raw, result.Raw)
				assert.Equal(t, tt.expected.Negated, result.Negated)
				assert.Equal(t, tt.expected.DirOnly, result.DirOnly)
				assert.Equal(t, tt.expected.Recursive, result.Recursive)
				assert.NotNil(t, result.Regex)
			}
		})
	}
}

func TestPatternMatches(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		isDir    bool
		expected bool
	}{
		{
			name:     "wildcard matches file",
			pattern:  "*.txt",
			path:     "test.txt",
			isDir:    false,
			expected: true,
		},
		{
			name:     "wildcard doesn't match directory",
			pattern:  "*.txt",
			path:     "test.txt",
			isDir:    true,
			expected: false,
		},
		{
			name:     "directory only matches directory",
			pattern:  "logs/",
			path:     "logs",
			isDir:    true,
			expected: true,
		},
		{
			name:     "directory only doesn't match file",
			pattern:  "logs/",
			path:     "logs",
			isDir:    false,
			expected: false,
		},
		{
			name:     "recursive pattern matches nested",
			pattern:  "**/cache",
			path:     "deep/nested/cache",
			isDir:    true,
			expected: true,
		},
		{
			name:     "absolute pattern matches root",
			pattern:  "/config.json",
			path:     "config.json",
			isDir:    false,
			expected: true,
		},
		{
			name:     "absolute pattern doesn't match nested",
			pattern:  "/config.json",
			path:     "subdir/config.json",
			isDir:    false,
			expected: false,
		},
		{
			name:     "question mark matches single char",
			pattern:  "test?.txt",
			path:     "test1.txt",
			isDir:    false,
			expected: true,
		},
		{
			name:     "negated pattern",
			pattern:  "!important.txt",
			path:     "important.txt",
			isDir:    false,
			expected: true, // Still matches, negation is handled at matcher level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := parsePattern(tt.pattern, 1)
			require.NoError(t, err)

			result := pattern.matches(tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePatternsFromReader(t *testing.T) {
	content := `
# This is a comment
*.txt
!important.txt
node_modules/
temp/
**/cache
`

	reader := strings.NewReader(content)
	patternSet, err := ParsePatternsFromReader(reader, "test")
	require.NoError(t, err)

	assert.Len(t, patternSet.GetPatterns(), 5)
	assert.Contains(t, patternSet.GetSources(), "test")
}

func TestParsePatternsFromFile(t *testing.T) {
	// Create a temporary file with patterns
	tmpDir := t.TempDir()
	patternFile := filepath.Join(tmpDir, ".testignore")
	content := `*.log
.DS_Store
/secret.txt
`
	err := os.WriteFile(patternFile, []byte(content), 0644)
	require.NoError(t, err)

	patternSet, err := ParsePatterns(patternFile)
	require.NoError(t, err)

	assert.Len(t, patternSet.GetPatterns(), 3)
	assert.Contains(t, patternSet.GetSources(), patternFile)
}

func TestLoadDefaultPatterns(t *testing.T) {
	patternSet := LoadDefaultPatterns()

	assert.False(t, patternSet.IsEmpty())
	assert.Greater(t, patternSet.Size(), 5)

	patterns := patternSet.GetPatterns()

	// Check that some default patterns exist
	var hasDSStore, hasGit, hasTmp bool
	for _, p := range patterns {
		if p.Raw == ".DS_Store" {
			hasDSStore = true
		}
		if p.Raw == ".git/" {
			hasGit = true
		}
		if p.Raw == "*.tmp" {
			hasTmp = true
		}
	}

	assert.True(t, hasDSStore, "Should have .DS_Store pattern")
	assert.True(t, hasGit, "Should have .git/ pattern")
	assert.True(t, hasTmp, "Should have *.tmp pattern")
}

func TestPatternSetOperations(t *testing.T) {
	set := NewPatternSet()

	// Test initial state
	assert.True(t, set.IsEmpty())
	assert.Equal(t, 0, set.Size())

	// Add patterns
	err := set.AddPattern("*.txt")
	require.NoError(t, err)

	err = set.AddPattern("!important.txt")
	require.NoError(t, err)

	// Test state after adding
	assert.False(t, set.IsEmpty())
	assert.Equal(t, 2, set.Size())

	// Test merge
	other := NewPatternSet()
	other.AddPattern("*.log")
	other.AddPattern("temp/")

	set.Merge(other)

	assert.Equal(t, 4, set.Size())
}

func TestNewMatcher(t *testing.T) {
	patternSet := LoadDefaultPatterns()
	matcher := NewMatcher(patternSet)

	assert.NotNil(t, matcher)
	assert.Equal(t, patternSet, matcher.GetPatternSet())
	assert.Equal(t, "", matcher.GetRootDir())
}

func TestNewMatcherWithRoot(t *testing.T) {
	patternSet := LoadDefaultPatterns()
	rootDir := "/test/path"
	matcher := NewMatcherWithRoot(patternSet, rootDir)

	assert.Equal(t, rootDir, matcher.GetRootDir())
}

func TestMatcherShouldExclude(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")
	patternSet.AddPattern("*.log")

	matcher := NewMatcher(patternSet)

	tests := []struct {
		name     string
		path     string
		isDir    bool
		expected bool
	}{
		{
			name:     "txt file excluded",
			path:     "test.txt",
			isDir:    false,
			expected: true,
		},
		{
			name:     "log file excluded",
			path:     "error.log",
			isDir:    false,
			expected: true,
		},
		{
			name:     "go file not excluded",
			path:     "main.go",
			isDir:    false,
			expected: false,
		},
		{
			name:     "directory not excluded by *.txt",
			path:     "test.txt",
			isDir:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.ShouldExclude(tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcherNegatedPatterns(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")
	patternSet.AddPattern("!important.txt")

	matcher := NewMatcher(patternSet)

	// important.txt should not be excluded due to negation
	excluded := matcher.ShouldExclude("important.txt", false)
	assert.False(t, excluded)

	// other.txt should still be excluded
	excluded = matcher.ShouldExclude("other.txt", false)
	assert.True(t, excluded)
}

func TestMatcherWithRootDir(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")

	matcher := NewMatcherWithRoot(patternSet, "/root/path")

	// File in root directory should be matched
	excluded := matcher.ShouldExclude("/root/path/test.txt", false)
	assert.True(t, excluded)

	// File in subdirectory should be matched (gitignore behavior)
	excluded = matcher.ShouldExclude("/root/path/sub/test.txt", false)
	assert.True(t, excluded)
}

func TestMatcherFilterFiles(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")
	patternSet.AddPattern("*.log")

	matcher := NewMatcher(patternSet)

	files := []string{
		"main.go",
		"test.txt",
		"error.log",
		"README.md",
	}

	filtered := matcher.FilterFiles(files)

	expected := []string{
		"main.go",
		"README.md",
	}

	assert.Equal(t, expected, filtered)
}

func TestMatcherFilterDirs(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("temp/")
	patternSet.AddPattern("cache/")

	matcher := NewMatcher(patternSet)

	dirs := []string{
		"src",
		"temp",
		"cache",
		"docs",
	}

	filtered := matcher.FilterDirs(dirs)

	expected := []string{
		"src",
		"docs",
	}

	assert.Equal(t, expected, filtered)
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test when no ignore file exists
	patternSet, err := LoadFromFile(tmpDir)
	require.NoError(t, err)
	assert.True(t, patternSet.IsEmpty())

	// Create ignore file
	ignoreFile := filepath.Join(tmpDir, ".nextcloudignore")
	content := `*.tmp
temp/
!keep.tmp
`
	err = os.WriteFile(ignoreFile, []byte(content), 0644)
	require.NoError(t, err)

	// Test loading ignore file
	patternSet, err = LoadFromFile(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 3, patternSet.Size())
}

func TestLoadFromFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple ignore files
	ignoreFile1 := filepath.Join(tmpDir, ".ignore1")
	ignoreFile2 := filepath.Join(tmpDir, ".ignore2")

	err := os.WriteFile(ignoreFile1, []byte("*.txt\n*.log\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(ignoreFile2, []byte("temp/\n*.tmp\n"), 0644)
	require.NoError(t, err)

	files := []string{ignoreFile1, ignoreFile2}
	patternSet, err := LoadFromFiles(files)
	require.NoError(t, err)
	assert.Equal(t, 4, patternSet.Size())
}

func TestMatcherClone(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")

	matcher := NewMatcherWithRoot(patternSet, "/test")
	clone := matcher.Clone()

	// Verify clone has same properties
	assert.Equal(t, matcher.GetPatternSet().Size(), clone.GetPatternSet().Size())
	assert.Equal(t, matcher.GetRootDir(), clone.GetRootDir())

	// Modify original and verify clone is unchanged
	matcher.SetRootDir("/different")
	assert.NotEqual(t, matcher.GetRootDir(), clone.GetRootDir())
}
