package source

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

// Target resolves a source ref into a materialized directory on disk that a
// scanner can read, plus a cleanup function to remove the directory when done.
type Target interface {
	Materialize(ctx context.Context, ref string) (dir string, cleanup func() error, err error)
}

// GitTarget materializes a git ref into a temporary directory by walking the
// commit tree at that ref with go-git. No subprocess invocation is used.
type GitTarget struct {
	repoPath string
}

// NewGitTarget returns a GitTarget rooted at the given repository path.
func NewGitTarget(repoPath string) *GitTarget {
	return &GitTarget{repoPath: repoPath}
}

// Materialize opens the repository, resolves ref to a commit, and writes every
// file in that commit's tree into a fresh temporary directory. The returned
// cleanup function removes the directory; callers must always call it (even on
// error) to avoid leaking disk space.
func (g *GitTarget) Materialize(ctx context.Context, ref string) (string, func() error, error) {
	noopCleanup := func() error { return nil }

	r, err := git.OpenRepository(g.repoPath)
	if err != nil {
		return "", noopCleanup, fmt.Errorf("open repo %q: %w", g.repoPath, err)
	}

	commit, err := resolveCommit(r, ref)
	if err != nil {
		return "", noopCleanup, fmt.Errorf("resolve ref %q: %w", ref, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", noopCleanup, fmt.Errorf("get tree for commit %s: %w", commit.Hash, err)
	}

	dir, err := os.MkdirTemp("", "chronicle-source-*")
	if err != nil {
		return "", noopCleanup, fmt.Errorf("create temp dir: %w", err)
	}

	cleanup := func() error {
		return os.RemoveAll(dir)
	}

	if err := materializeTree(ctx, tree, dir); err != nil {
		// best-effort cleanup on failure; callers may also call the returned cleanup
		_ = os.RemoveAll(dir)
		return "", noopCleanup, fmt.Errorf("materialize tree for ref %q: %w", ref, err)
	}

	log.WithFields("ref", ref, "commit", commit.Hash.String(), "dir", dir).Debug("materialized git ref into temp directory")

	return dir, cleanup, nil
}

// resolveCommit resolves a ref to a commit. It tries a tag reference first
// (both lightweight and annotated, which ResolveRevision does not reliably find
// or peel), then falls back to go-git's revision resolution for HEAD, branch
// names, and full/short commit SHAs.
func resolveCommit(r *gogit.Repository, ref string) (*object.Commit, error) {
	if tagRef, err := r.Reference(plumbing.NewTagReferenceName(ref), false); err == nil && tagRef != nil {
		return commitFromTagRef(r, tagRef)
	}

	hash, err := r.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, err
	}
	return r.CommitObject(*hash)
}

// commitFromTagRef peels a tag reference down to its commit. A lightweight tag's
// hash points directly at the commit; an annotated tag's hash points at a tag
// object that targets the commit.
func commitFromTagRef(r *gogit.Repository, t *plumbing.Reference) (*object.Commit, error) {
	if c, err := r.CommitObject(t.Hash()); err == nil && c != nil {
		return c, nil
	}

	tagObj, err := object.GetTag(r.Storer, t.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve annotated tag %q: %w", t.Name().Short(), err)
	}
	return r.CommitObject(tagObj.Target)
}

// materializeTree walks every file in the given tree and writes it under dir,
// preserving relative paths. Submodule entries (mode 0160000) are skipped
// silently since they have no blob content. Context cancellation is honoured
// between files.
func materializeTree(ctx context.Context, tree *object.Tree, dir string) error {
	files := tree.Files()
	defer files.Close()

	return files.ForEach(func(f *object.File) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		// skip submodule gitlinks — they carry no blob data
		if f.Mode == 0160000 {
			log.WithFields("path", f.Name).Trace("skipping submodule entry during tree materialization")
			return nil
		}

		dest := filepath.Join(dir, filepath.FromSlash(f.Name))

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create directory for %q: %w", f.Name, err)
		}

		if err := writeBlob(f, dest); err != nil {
			return fmt.Errorf("write file %q: %w", f.Name, err)
		}

		return nil
	})
}

// writeBlob streams the content of a git file object to the given destination path.
func writeBlob(f *object.File, dest string) error {
	rc, err := f.Reader()
	if err != nil {
		return fmt.Errorf("open blob reader: %w", err)
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("copy blob content: %w", err)
	}

	return nil
}
