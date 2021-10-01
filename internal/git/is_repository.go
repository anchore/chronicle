package git

import (
	"github.com/go-git/go-git/v5"
)

func IsRepository(path string) bool {
	r, err := git.PlainOpen(path)
	if err != nil {
		return false
	}
	return r != nil
}
