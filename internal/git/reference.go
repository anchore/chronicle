package git

type Reference struct {
	Commit string
	Tags   []string
}

// func commitFromGitReference(repoPath, name string) (string, error) {
//	r, err := git.PlainOpen(repoPath)
//	if err != nil {
//		return "", err
//
//	}
//
//	ref, err := r.Reference(plumbing.ReferenceName(path.Join("refs", "tags", name)), false)
//	if err != nil {
//		return "", err
//	}
//
//	if ref != nil {
//		return ref.String(), nil
//	}
//
//	tags, err := TagsFromLocal(repoPath)
//	if err != nil {
//		return "", err
//	}
//
//	var commit string
//	for _, tag := range tags {
//		if tag.Name == sinceRef {
//			r.Tag()
//		}
//	}
//}
