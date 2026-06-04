package git

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FileBlob is a single file's path and content as it existed at a particular git ref.
type FileBlob struct {
	Path    string // path relative to the repo root, slash-separated
	Content []byte
}

// ListFilesAtRef walks the tree at the given ref and returns the path and content of every file
// for which match returns true (a nil match selects all files). Content is read straight from the
// object store, so no working-tree checkout is performed.
func ListFilesAtRef(repoPath, ref string, match func(path string) bool) ([]FileBlob, error) {
	r, err := openRepo(repoPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open repo %q: %w", repoPath, err)
	}

	hash, err := r.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("unable to resolve git ref=%q: %w", ref, err)
	}

	commit, err := r.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("unable to load commit for ref=%q: %w", ref, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("unable to load tree for ref=%q: %w", ref, err)
	}

	var out []FileBlob
	err = tree.Files().ForEach(func(f *object.File) error {
		if match != nil && !match(f.Name) {
			return nil
		}
		content, err := f.Contents()
		if err != nil {
			return fmt.Errorf("unable to read %q at ref=%q: %w", f.Name, ref, err)
		}
		out = append(out, FileBlob{Path: f.Name, Content: []byte(content)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
