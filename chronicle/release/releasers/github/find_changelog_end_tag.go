package github

import (
	"fmt"
	"strings"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func FindChangelogEndTag(summer release.Summarizer, gitter git.Interface) (string, error) {
	// check if the current commit is tagged, then use that
	currentTag, err := gitter.HeadTag()
	if err != nil {
		return "", fmt.Errorf("problem while attempting to find HEAD tag: %w", err)
	}
	if currentTag == "" {
		return "", nil
	}

	taggedRelease, err := summer.Release(currentTag)
	if err != nil {
		// the GitHub GraphQL API returns this message body when the release for a given tag does not exist;
		// only treat that specific case as "no release yet" and propagate everything else.
		if isReleaseNotFoundErr(err) {
			log.WithFields("tag", currentTag).Debug("no GitHub release exists for HEAD tag yet")
			return currentTag, nil
		}
		return "", fmt.Errorf("unable to fetch release for tag %q: %w", currentTag, err)
	}
	if taggedRelease != nil {
		log.WithFields("tag", currentTag).Info("HEAD tag already has an associated GitHub release; ignoring")
		return "", nil
	}

	log.WithFields("tag", currentTag).Info("found tag at HEAD without an associated GitHub release")

	// a tag was found and there is no existing release for this tag
	return currentTag, nil
}

func isReleaseNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// known GitHub GraphQL responses for missing release lookups
	return strings.Contains(msg, "Could not resolve to a Release") ||
		strings.Contains(msg, "Could not resolve to a node with the global id")
}
