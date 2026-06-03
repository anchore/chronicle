package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anchore/chronicle/internal"
)

var remotePattern = regexp.MustCompile(`\[remote\s*"origin"]\s*\n\s*url\s*=\s*(?P<url>[^\s]+)\s+`)

// TODO: can't use r.Config for same validation reasons
func RemoteURL(p string) (string, error) {
	cfgPath, err := gitConfigPath(p)
	if err != nil {
		return "", err
	}
	f, err := os.Open(cfgPath)
	if err != nil {
		return "", fmt.Errorf("unable to open git config %q: %w", cfgPath, err)
	}
	contents, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("unable to read git config %q: %w", cfgPath, err)
	}
	matches := internal.MatchNamedCaptureGroups(remotePattern, string(contents))

	url := matches["url"]
	if url == "" {
		return "", fmt.Errorf("no 'origin' remote URL found in %q (chronicle requires an 'origin' remote pointing to GitHub)", cfgPath)
	}

	return url, nil
}

// gitConfigPath resolves the path to the git config file for the repo rooted at p. For a normal
// repo this is simply ".git/config", but when worktrees are in use ".git" is a file pointing at a
// separate git dir, so we must follow that pointer to locate the shared config.
func gitConfigPath(p string) (string, error) {
	dotGit := filepath.Join(p, ".git")
	fi, err := os.Stat(dotGit)
	if err != nil {
		return "", fmt.Errorf("unable to stat %q: %w", dotGit, err)
	}

	// common case: .git is a directory holding the config directly
	if fi.IsDir() {
		return filepath.Join(dotGit, "config"), nil
	}

	// worktree/submodule case: .git is a file containing a "gitdir:" pointer to the real git dir
	gitDir, err := readGitDirPointer(dotGit)
	if err != nil {
		return "", err
	}

	// worktrees keep per-worktree metadata in their own git dir but share config via the common
	// dir, which is recorded in a "commondir" file (relative paths resolve against the git dir).
	commonDir := gitDir
	if data, readErr := os.ReadFile(filepath.Join(gitDir, "commondir")); readErr == nil {
		common := strings.TrimSpace(string(data))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
		commonDir = common
	}

	return filepath.Join(commonDir, "config"), nil
}

// readGitDirPointer reads a ".git" file and returns the path it points at via its "gitdir:" line,
// resolving relative pointers against the directory containing the file.
func readGitDirPointer(dotGitFile string) (string, error) {
	data, err := os.ReadFile(dotGitFile)
	if err != nil {
		return "", fmt.Errorf("unable to read git dir pointer %q: %w", dotGitFile, err)
	}

	const prefix = "gitdir:"
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(filepath.Dir(dotGitFile), gitDir)
		}
		return gitDir, nil
	}

	return "", fmt.Errorf("no 'gitdir:' pointer found in %q", dotGitFile)
}

// TODO: can't use r.Config for same validation reasons
// func RemoteURL(path string) (string, error) {
//	r, err := git.PlainOpen(path)
//	if err != nil {
//		return "", fmt.Errorf("unable to open repo: %w", err)
//	}
//	c, err := r.Config()
//	if err != nil {
//		return "", fmt.Errorf("unable to get config: %+v", err)
//	}
//
//	for _, section := range c.Raw.Sections {
//		if section.Name == "remote" {
//			for _, subsection := range section.Subsections {
//				// TODO: make configurable
//				if subsection.Name == "origin" {
//					for _, option := range subsection.Options {
//						if option.Key == "url" {
//							return option.Value, nil
//						}
//					}
//				}
//			}
//		}
//	}
//
//	return "", fmt.Errorf("unable to find origin url")
//}

// TODO: it seems that this lib has a config validation problem :(
// func RemoteURL(path string) (string, error)  {
//	r, err := git.PlainOpen(path)
//	if err != nil {
//		return "", fmt.Errorf("unable to open repo: %w", err)
//	}
//
//	remotes, err := r.Remotes()
//	if err != nil {
//		return "", fmt.Errorf("unable to list repo remotes: %w", err)
//	}
//
//	var repoUrl string
//	for _, remote := range remotes {
//		// TODO: this shouldn't be so absolutist about the origin ref
//		if remote.Config().Name == "origin" {
//			for _, url := range remote.Config().URLs {
//				// TODO: doesn't support enterprise instances
//				if strings.Contains(url, "github.com") {
//					repoUrl = url
//				}
//			}
//		}
//	}
//
//	if repoUrl == "" {
//		return "", errors.New("failed to find repo URL")
//	}
//	return repoUrl, nil
//}
