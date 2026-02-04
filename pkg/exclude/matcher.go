package exclude

import (
	"os"
	"path/filepath"
	"strings"
)

// Matcher provides functionality to match file paths against exclusion patterns
type Matcher struct {
	patternSet *PatternSet
	rootDir    string // Root directory for relative path calculations
}

// NewMatcher creates a new matcher with the given pattern set
func NewMatcher(patternSet *PatternSet) *Matcher {
	return &Matcher{
		patternSet: patternSet,
		rootDir:    "",
	}
}

// NewMatcherWithRoot creates a new matcher with a root directory
func NewMatcherWithRoot(patternSet *PatternSet, rootDir string) *Matcher {
	return &Matcher{
		patternSet: patternSet,
		rootDir:    rootDir,
	}
}

// ShouldExclude determines if a file/directory should be excluded
func (m *Matcher) ShouldExclude(path string, isDir bool) bool {
	if m.patternSet == nil || m.patternSet.IsEmpty() {
		return false
	}

	// Get the relative path from root if rootDir is set
	relativePath := m.getRelativePath(path)

	// Check against each pattern
	excluded := false

	for _, pattern := range m.patternSet.GetPatterns() {
		matched := pattern.matches(relativePath, isDir)

		if matched {
			if pattern.Negated {
				// Negated pattern un-excludes
				excluded = false
			} else {
				// Regular pattern excludes
				excluded = true
			}
		}
	}

	return excluded
}

// ShouldExcludeFile determines if a file should be excluded
func (m *Matcher) ShouldExcludeFile(path string) bool {
	isDir := false
	return m.ShouldExclude(path, isDir)
}

// ShouldExcludeDir determines if a directory should be excluded
func (m *Matcher) ShouldExcludeDir(path string) bool {
	isDir := true
	return m.ShouldExclude(path, isDir)
}

// getRelativePath returns the path relative to the root directory
func (m *Matcher) getRelativePath(path string) string {
	if m.rootDir == "" {
		// No root directory, use path as-is with forward slashes
		return strings.ReplaceAll(path, "\\", "/")
	}

	// Convert to forward slashes for consistency
	path = filepath.ToSlash(path)
	rootDir := filepath.ToSlash(m.rootDir)

	// Remove root directory prefix if present
	if strings.HasPrefix(path, rootDir) {
		if len(path) > len(rootDir) && path[len(rootDir)] == '/' {
			return path[len(rootDir)+1:]
		} else if len(path) == len(rootDir) {
			return ""
		}
	}

	return path
}

// hasPathDepth checks if the path has multiple levels (contains '/')
func (m *Matcher) hasPathDepth(path string) bool {
	return strings.Contains(path, "/")
}

// matches checks if a pattern matches the given path
func (p *Pattern) matches(path string, isDir bool) bool {
	// Directory-only patterns only match directories
	if p.DirOnly && !isDir {
		return false
	}

	// Non-directory patterns should not match directories unless they are recursive
	if !p.DirOnly && isDir && !p.Recursive {
		return false
	}

	// Normalize path for matching - ensure forward slashes
	path = strings.ReplaceAll(path, "\\", "/")

	// Use the compiled regex to match
	return p.Regex.MatchString(path)
}

// FilterFiles filters a list of files, returning only those that are not excluded
func (m *Matcher) FilterFiles(files []string) []string {
	if m.patternSet == nil || m.patternSet.IsEmpty() {
		return files
	}

	var filtered []string
	for _, file := range files {
		if !m.ShouldExcludeFile(file) {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// FilterDirs filters a list of directories, returning only those that are not excluded
func (m *Matcher) FilterDirs(dirs []string) []string {
	if m.patternSet == nil || m.patternSet.IsEmpty() {
		return dirs
	}

	var filtered []string
	for _, dir := range dirs {
		if !m.ShouldExcludeDir(dir) {
			filtered = append(filtered, dir)
		}
	}

	return filtered
}

// WalkFunc is the type of function called for each file or directory visited by Walk
type WalkFunc func(path string, info os.FileInfo, err error) error

// Walk walks the file tree rooted at root, calling walkFn for each file or directory
// in the tree, including root. All paths that match patterns are skipped.
func (m *Matcher) Walk(root string, walkFn WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return walkFn(path, info, err)
		}

		// Check if current path should be excluded (ShouldExclude handles relative path conversion)
		shouldExclude := m.ShouldExclude(path, info.IsDir())

		// If this is a directory that should be excluded, skip it entirely
		if info.IsDir() && shouldExclude && path != root {
			return filepath.SkipDir
		}

		// For files that should be excluded, don't call walkFn
		if !info.IsDir() && shouldExclude {
			return nil // Skip this file
		}

		// Call the walk function for non-excluded paths
		return walkFn(path, info, nil)
	})
}

// GetExcludedPaths returns a list of paths that would be excluded
func (m *Matcher) GetExcludedPaths(root string) ([]string, error) {
	var excluded []string

	// Use filepath.Walk directly to check all paths, not m.Walk which skips excluded paths
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Convert path to relative path for checking
		relativePath := path
		if m.rootDir != "" {
			if rel, err := filepath.Rel(m.rootDir, path); err == nil {
				relativePath = rel
			}
		}
		if relativePath == "." {
			relativePath = ""
		}

		// Check if this path would be excluded
		shouldExclude := m.ShouldExclude(relativePath, info.IsDir())
		if shouldExclude {
			excluded = append(excluded, relativePath)
		}

		return nil
	})

	return excluded, err
}

// GetIncludedPaths returns a list of paths that would NOT be excluded
func (m *Matcher) GetIncludedPaths(root string) ([]string, error) {
	var included []string

	err := m.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Convert path to relative path for checking
		relativePath := path
		if m.rootDir != "" {
			if rel, err := filepath.Rel(m.rootDir, path); err == nil {
				relativePath = rel
			}
		}

		// Check if this path would be included (not excluded)
		shouldExclude := m.ShouldExclude(relativePath, info.IsDir())
		if !shouldExclude {
			included = append(included, relativePath)
		}

		return nil
	})

	return included, err
}

// SetRootDir sets the root directory for relative path calculations
func (m *Matcher) SetRootDir(rootDir string) {
	m.rootDir = rootDir
}

// GetRootDir returns the current root directory
func (m *Matcher) GetRootDir() string {
	return m.rootDir
}

// GetPatternSet returns the pattern set used by this matcher
func (m *Matcher) GetPatternSet() *PatternSet {
	return m.patternSet
}

// Clone creates a copy of the matcher
func (m *Matcher) Clone() *Matcher {
	// Create new pattern set with same patterns
	newPatternSet := NewPatternSet()
	if m.patternSet != nil {
		newPatternSet.Merge(m.patternSet)
	}

	return &Matcher{
		patternSet: newPatternSet,
		rootDir:    m.rootDir,
	}
}
