package git

import (
	"fmt"
	"io"
	"os"
	"path"
	"regexp"

	"github.com/anchore/chronicle/internal"
)

var remotePattern = regexp.MustCompile(`(?m)\[remote "origin"](\n.*)*url\s*=\s*(?P<url>.*)$`)

// TODO: can't use r.Config for same validation reasons
func RemoteURL(p string) (string, error) {
	f, err := os.Open(path.Join(p, ".git", "config"))
	if err != nil {
		return "", fmt.Errorf("unable to open git config: %w", err)
	}
	contents, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("unable to read git config: %w", err)
	}
	matches := internal.MatchNamedCaptureGroups(remotePattern, string(contents))

	return matches["url"], nil
}

// TODO: can't use r.Config for same validation reasons
//func RemoteURL(path string) (string, error) {
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
//func RemoteURL(path string) (string, error)  {
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
