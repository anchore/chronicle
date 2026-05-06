package git

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/anchore/chronicle/internal/log"
)

// errStopIter is a sentinel used to short-circuit a tag-ref iteration once we've found a match.
var errStopIter = errors.New("stop iteration")

func HeadTagOrCommit(repoPath string) (string, error) {
	return headTag(repoPath, true)
}

func HeadTag(repoPath string) (string, error) {
	return headTag(repoPath, false)
}

func headTag(repoPath string, orCommit bool) (string, error) {
	r, err := openRepo(repoPath)
	if err != nil {
		return "", fmt.Errorf("unable to open repo %q: %w", repoPath, err)
	}
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("unable to fetch HEAD reference: %w", err)
	}

	tagRefs, err := r.Tags()
	if err != nil {
		return "", fmt.Errorf("unable to enumerate tags: %w", err)
	}

	var tagName string
	iterErr := tagRefs.ForEach(func(t *plumbing.Reference) error {
		if t.Hash().String() == ref.Hash().String() {
			// for lightweight tags, the tag hash points directly to the commit
			tagName = t.Name().Short()
			return errStopIter
		}

		// for annotated tags we need to resolve the revision to get the commit the tag object points at
		revHash, err := r.ResolveRevision(plumbing.Revision(t.Name()))
		if err != nil {
			// some refs may not resolve (e.g. dangling tag objects); record and continue rather than fail the whole walk
			log.WithFields("ref", t.Name().String(), "error", err).Trace("skipping unresolvable tag ref")
			return nil
		}

		if revHash == nil {
			return nil
		}

		if *revHash == ref.Hash() {
			tagName = t.Name().Short()
			return errStopIter
		}
		return nil
	})
	if iterErr != nil && !errors.Is(iterErr, errStopIter) {
		return "", fmt.Errorf("error while walking tag references: %w", iterErr)
	}

	if tagName != "" {
		return tagName, nil
	}

	if orCommit {
		return ref.Hash().String(), nil
	}
	return "", nil
}

func HeadCommit(repoPath string) (string, error) {
	r, err := openRepo(repoPath)
	if err != nil {
		return "", fmt.Errorf("unable to open repo %q: %w", repoPath, err)
	}
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("unable to fetch HEAD reference: %w", err)
	}
	return ref.Hash().String(), nil
}
