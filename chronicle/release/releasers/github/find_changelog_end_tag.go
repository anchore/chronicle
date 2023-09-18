package github

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func FindChangelogEndTag(summer release.Summarizer, gitter git.Interface) (string, error) {
	// check if the current commit is tagged, then use that
	currentTag, err := gitter.HeadTag()
	if err != nil {
		return "", fmt.Errorf("problem while attempting to find head tag: %w", err)
	}
	if currentTag == "" {
		return "", nil
	}

	if taggedRelease, err := summer.Release(currentTag); err != nil {
		// TODO: assert the error specifically confirms that the release does not exist, not just any error
		// no release found, assume that this is the correct release info
		return "", fmt.Errorf("unable to fetch release=%q : %w", currentTag, err)
	} else if taggedRelease != nil {
		log.WithFields("tag", currentTag).Debug("found existing tag however, it already has an associated release. ignoring...")
		// return commitRef, nil
		return "", nil
	}

	log.WithFields("tag", currentTag).Debug("found existing tag at HEAD which does not have an associated release")

	// a tag was found and there is no existing release for this tag
	return currentTag, nil
}
