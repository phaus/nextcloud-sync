package exclude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcherWalk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	dirs := []string{
		"src",
		"src/subdir",
		"temp",
		"logs",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create test files
	files := []string{
		"main.go",
		"test.txt",
		"error.log",
		"src/app.go",
		"src/subdir/helper.go",
		"src/subdir/readme.txt",
		"temp/cache.tmp",
		"logs/app.log",
	}

	for _, file := range files {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Create pattern set to exclude .txt and .log files
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")
	patternSet.AddPattern("*.log")

	matcher := NewMatcherWithRoot(patternSet, tmpDir)

	// Walk and collect visited paths
	var visited []string
	err := matcher.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Convert to relative path for checking
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)

		if relPath != "." {
			visited = append(visited, relPath)
		}
		return nil
	})

	require.NoError(t, err)

	// Verify that excluded files are not visited
	assert.Contains(t, visited, "main.go")
	assert.NotContains(t, visited, "test.txt")
	assert.NotContains(t, visited, "error.log")
	assert.Contains(t, visited, "src/app.go")
	assert.NotContains(t, visited, "src/subdir/readme.txt")
	assert.NotContains(t, visited, "logs/app.log")
}

func TestMatcherWalkWithDirExclusion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	dirs := []string{
		"src",
		"temp",
		"temp/nested",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create files
	files := []string{
		"src/main.go",
		"temp/cache.tmp",
		"temp/nested/deep.txt",
		"keep.txt",
	}

	for _, file := range files {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Create pattern set to exclude temp directory
	patternSet := NewPatternSet()
	patternSet.AddPattern("temp/")

	matcher := NewMatcherWithRoot(patternSet, tmpDir)

	// Walk and collect visited paths
	var visited []string
	err := matcher.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)

		if relPath != "." {
			visited = append(visited, relPath)
		}
		return nil
	})

	require.NoError(t, err)

	// Verify that temp directory and its contents are not visited
	assert.Contains(t, visited, "src/main.go")
	assert.Contains(t, visited, "keep.txt")
	assert.NotContains(t, visited, "temp")
	assert.NotContains(t, visited, "temp/cache.tmp")
	assert.NotContains(t, visited, "temp/nested/deep.txt")
}

func TestMatcherGetExcludedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "src", "app.go"), []byte("package app"), 0644)
	require.NoError(t, err)

	// Create pattern set
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")
	patternSet.AddPattern("src/")

	matcher := NewMatcherWithRoot(patternSet, tmpDir)

	// Get excluded paths
	excluded, err := matcher.GetExcludedPaths(tmpDir)
	require.NoError(t, err)

	assert.Contains(t, excluded, "test.txt")
	assert.Contains(t, excluded, "src")
}

func TestMatcherGetIncludedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "src", "app.go"), []byte("package app"), 0644)
	require.NoError(t, err)

	// Create pattern set
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")

	matcher := NewMatcherWithRoot(patternSet, tmpDir)

	// Get included paths
	included, err := matcher.GetIncludedPaths(tmpDir)
	require.NoError(t, err)

	assert.NotContains(t, included, "test.txt")
	assert.Contains(t, included, "main.go")
	assert.Contains(t, included, "src")
	assert.Contains(t, included, "src"+string(filepath.Separator)+"app.go")
}

func TestMatcherShouldExcludeFile(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")
	patternSet.AddPattern("*.log")

	matcher := NewMatcher(patternSet)

	// Test file exclusion (ignores isDir=false)
	assert.True(t, matcher.ShouldExcludeFile("test.txt"))
	assert.True(t, matcher.ShouldExcludeFile("error.log"))
	assert.False(t, matcher.ShouldExcludeFile("main.go"))
}

func TestMatcherShouldExcludeDir(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("temp/")
	patternSet.AddPattern("*.txt") // This should not match directories

	matcher := NewMatcher(patternSet)

	// Test directory exclusion
	assert.True(t, matcher.ShouldExcludeDir("temp"))
	assert.False(t, matcher.ShouldExcludeDir("test.txt")) // *.txt shouldn't match directories
	assert.False(t, matcher.ShouldExcludeDir("src"))
}

func TestMatcherSetRootDir(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("*.txt")

	matcher := NewMatcher(patternSet)

	// Initially no root dir
	assert.Equal(t, "", matcher.GetRootDir())

	// Set root dir
	matcher.SetRootDir("/test/path")
	assert.Equal(t, "/test/path", matcher.GetRootDir())

	// Test that root dir affects matching
	assert.True(t, matcher.ShouldExclude("/test/path/test.txt", false))
}

func TestMatcherComplexPatterns(t *testing.T) {
	patternSet := NewPatternSet()
	patternSet.AddPattern("**/*.tmp")       // Recursive
	patternSet.AddPattern("/config.json")   // Absolute
	patternSet.AddPattern("build/")         // Directory only
	patternSet.AddPattern("!important.tmp") // Negated

	matcher := NewMatcher(patternSet)

	// Test recursive pattern
	assert.True(t, matcher.ShouldExclude("cache/file.tmp", false))
	assert.True(t, matcher.ShouldExclude("deep/nested/cache/file.tmp", false))

	// Test absolute pattern
	assert.True(t, matcher.ShouldExclude("config.json", false))
	assert.False(t, matcher.ShouldExclude("subdir/config.json", false))

	// Test directory-only pattern
	assert.True(t, matcher.ShouldExcludeDir("build"))
	assert.False(t, matcher.ShouldExclude("build", false)) // File named "build"

	// Test negated pattern (important.tmp should not be excluded)
	assert.False(t, matcher.ShouldExclude("important.tmp", false))
}

func TestMatcherEmptyPatternSet(t *testing.T) {
	patternSet := NewPatternSet()
	matcher := NewMatcher(patternSet)

	// Empty pattern set should not exclude anything
	assert.False(t, matcher.ShouldExclude("any.txt", false))
	assert.False(t, matcher.ShouldExcludeDir("anydir"))

	// Filter operations should return input unchanged
	files := []string{"a.txt", "b.log", "c.go"}
	filtered := matcher.FilterFiles(files)
	assert.Equal(t, files, filtered)

	dirs := []string{"src", "temp", "logs"}
	filteredDirs := matcher.FilterDirs(dirs)
	assert.Equal(t, dirs, filteredDirs)
}

func TestMatcherNilPatternSet(t *testing.T) {
	matcher := NewMatcher(nil)

	// Nil pattern set should not exclude anything
	assert.False(t, matcher.ShouldExclude("any.txt", false))
	assert.False(t, matcher.ShouldExcludeDir("anydir"))
}

func TestPatternToRegexEdgeCases(t *testing.T) {
	tests := []struct {
		pattern     string
		testPath    string
		shouldMatch bool
	}{
		{
			pattern:     "*.txt",
			testPath:    "test.txt",
			shouldMatch: true,
		},
		{
			pattern:     "test?",
			testPath:    "test1",
			shouldMatch: true,
		},
		{
			pattern:     "test?",
			testPath:    "test12",
			shouldMatch: false,
		},
		{
			pattern:     "src/**/*.go",
			testPath:    "src/main.go",
			shouldMatch: true,
		},
		{
			pattern:     "src/**/*.go",
			testPath:    "src/subdir/main.go",
			shouldMatch: true,
		},
		{
			pattern:     "**/node_modules/**",
			testPath:    "node_modules/package/index.js",
			shouldMatch: true,
		},
		{
			pattern:     "**/node_modules/**",
			testPath:    "src/node_modules/package/index.js",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+" with "+tt.testPath, func(t *testing.T) {
			regex, err := patternToRegex(tt.pattern, false, strings.Contains(tt.pattern, "**"))
			require.NoError(t, err)

			matches := regex.MatchString(tt.testPath)
			assert.Equal(t, tt.shouldMatch, matches)
		})
	}
}

func TestIsSpecialRegexChar(t *testing.T) {
	tests := []struct {
		char    rune
		special bool
	}{
		{'.', true},
		{'*', true},
		{'+', true},
		{'?', true},
		{'^', true},
		{'$', true},
		{'(', true},
		{')', true},
		{'[', true},
		{']', true},
		{'{', true},
		{'}', true},
		{'|', true},
		{'\\', true},
		{'a', false},
		{'b', false},
		{'1', false},
		{'_', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := isSpecialRegexChar(tt.char)
			assert.Equal(t, tt.special, result)
		})
	}
}
