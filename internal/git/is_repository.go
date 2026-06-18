package git

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	rawconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"

	"github.com/anchore/chronicle/internal/log"
)

// openRepo opens a git repository at the given path. It tries, in order:
//  1. a strict open at the path (the common case).
//  2. a fallback that walks parent directories and follows .git-as-file pointers
//     (handles git worktrees, submodules, and being run from a subdirectory).
//  3. a last-resort lenient open that bypasses go-git's overly strict branch
//     config validation. Real `git` accepts configs (e.g. branch.<name>.merge
//     pointing at non-branch refs) that go-git rejects with errors like
//     "branch config: invalid merge"; we skip those branch entries since
//     chronicle does not read branch tracking config.
//
// On failure the returned error contains the underlying go-git reason.
func openRepo(path string) (*gogit.Repository, error) {
	// EnableDotGitCommonDir is required for worktrees (and submodules): there ".git" is a file
	// pointing at a per-worktree git dir, while refs and objects live in the shared common dir. A
	// plain open would succeed but yield a repo whose HEAD can't resolve. We keep DetectDotGit off
	// here so the strict open still only considers the given path (preserving IsRepository semantics).
	if r, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{EnableDotGitCommonDir: true}); err == nil {
		return r, nil
	} else if !isConfigValidationErr(err) {
		// remember the strict error in case the fallbacks turn up nothing more useful
		strictErr := err

		if r, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{
			DetectDotGit:          true,
			EnableDotGitCommonDir: true,
		}); err == nil {
			return r, nil
		} else if !isConfigValidationErr(err) {
			if errors.Is(strictErr, gogit.ErrRepositoryNotExists) {
				return nil, strictErr
			}
			return nil, err
		}
	}

	// at least one open path failed with a config-validation error; attempt a lenient open
	r, lenErr := openLenient(path)
	if lenErr != nil {
		return nil, lenErr
	}
	return r, nil
}

// isConfigValidationErr reports whether the error came from go-git's config validator
// (rather than e.g. a missing repository or unreadable file).
func isConfigValidationErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "branch config:") ||
		strings.Contains(msg, "remote config:") ||
		errors.Is(err, gogitconfig.ErrInvalid)
}

// openLenient opens a repository while bypassing go-git's strict config validation by
// pre-sanitizing the .git/config bytes before they reach go-git's parser.
func openLenient(path string) (*gogit.Repository, error) {
	// resolve dot-git and worktree filesystems the same way PlainOpenWithOptions does, but
	// without going through the validating Open helper.
	dot, wt, err := resolveDotGit(path)
	if err != nil {
		return nil, err
	}

	configBytes, err := readFile(dot, "config")
	if err != nil {
		return nil, err
	}

	cleaned, dropped := stripInvalidBranches(configBytes)
	if len(dropped) > 0 {
		// emitted once per openRepo call, which can be many times per run — trace-only since the
		// underlying condition is benign (real git tolerates these branch configs; chronicle does
		// not read them at all).
		log.WithFields("branches", strings.Join(dropped, ",")).Trace("skipping branch config entries rejected by go-git's stricter validator")
	}

	storer := filesystem.NewStorage(dot, cache.NewObjectLRUDefault())
	lenient := &lenientStorage{Storage: storer, sanitizedConfig: cleaned}

	return gogit.Open(lenient, wt)
}

// lenientStorage wraps a filesystem.Storage and overrides Config() to return a config parsed from
// pre-sanitized bytes, side-stepping go-git's overly strict branch validation.
type lenientStorage struct {
	*filesystem.Storage
	sanitizedConfig []byte
}

func (l *lenientStorage) Config() (*gogitconfig.Config, error) {
	cfg := gogitconfig.NewConfig()
	if err := cfg.Unmarshal(l.sanitizedConfig); err != nil {
		// fall back to the original Storage.Config so the caller sees the real error
		return l.Storage.Config()
	}
	return cfg, nil
}

func (l *lenientStorage) SetConfig(cfg *gogitconfig.Config) error {
	return l.Storage.SetConfig(cfg)
}

// stripInvalidBranches removes [branch "X"] subsections whose `merge` value would fail
// go-git's Branch.Validate (Merge must be a branch ref). Real git accepts configs like
// `merge = refs/tags/...` or arbitrary strings; chronicle does not consult this metadata.
// Returns the cleaned config bytes and the names of any branches that were dropped.
func stripInvalidBranches(b []byte) ([]byte, []string) {
	raw := rawconfig.New()
	if err := rawconfig.NewDecoder(bytes.NewReader(b)).Decode(raw); err != nil {
		return b, nil
	}

	var dropped []string
	for _, section := range raw.Sections {
		if section.Name != "branch" {
			continue
		}
		kept := section.Subsections[:0]
		for _, sub := range section.Subsections {
			merge := plumbing.ReferenceName(sub.Options.Get("merge"))
			if merge != "" && !merge.IsBranch() {
				dropped = append(dropped, sub.Name)
				continue
			}
			kept = append(kept, sub)
		}
		section.Subsections = kept
	}

	var buf bytes.Buffer
	if err := rawconfig.NewEncoder(&buf).Encode(raw); err != nil {
		return b, nil
	}
	return buf.Bytes(), dropped
}

// resolveDotGit returns the dot-git and worktree filesystems for the repo at path, mirroring what
// go-git's PlainOpenWithOptions(EnableDotGitCommonDir) does but without going through its validating
// Open helper. It handles plain repos and the .git-as-file (gitdir pointer) layout, and for linked
// worktrees wires the per-worktree git dir together with the shared common dir so reads of config,
// refs, and objects route to the right place.
func resolveDotGit(path string) (dot, wt billy.Filesystem, err error) {
	wt = osfs.New(path)

	dirs, err := resolveGitDirs(path)
	if err != nil {
		return nil, nil, err
	}

	dot = osfs.New(dirs.gitDir)

	// for a linked worktree the git dir holds only per-worktree state (HEAD, index) while config,
	// refs, and objects live in the common dir; merge the two the same way go-git does.
	if dirs.commonDir != dirs.gitDir {
		dot = dotgit.NewRepositoryFilesystem(dot, osfs.New(dirs.commonDir))
	}

	return dot, wt, nil
}

func readFile(fs billy.Filesystem, name string) ([]byte, error) {
	f, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// IsRepository reports whether the given path can be opened as a git repository.
// Prefer openRepo / git.New when you also want the underlying error.
func IsRepository(path string) bool {
	_, err := openRepo(path)
	return err == nil
}

// OpenRepository opens the git repository at path and returns the underlying go-git
// handle. It applies the same worktree-aware, lenient-config handling as the rest of
// this package (see openRepo), so callers that need a raw *go-git.Repository — e.g. to
// walk a tree at a ref — get the same tolerance for branch configs that go-git's
// validator rejects but real git accepts.
func OpenRepository(path string) (*gogit.Repository, error) {
	return openRepo(path)
}
