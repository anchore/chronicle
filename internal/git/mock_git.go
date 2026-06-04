package git

var _ Interface = (*MockInterface)(nil)

type MockInterface struct {
	MockHeadOrTagCommit        string
	MockHeadTag                string
	MockTags                   []string
	MockRemoteURL              string
	MockSearchTag              string
	MockCommitsBetween         []string
	MockCommitsBetweenWithMeta []Commit
	MockFirstCommit            string
	MockFilesAtRef             map[string][]FileBlob // ref -> files present at that ref
	MockDirtyPaths             []string              // working-tree paths with uncommitted changes
}

func (m MockInterface) CommitsBetween(_ Range) ([]string, error) {
	return m.MockCommitsBetween, nil
}

func (m MockInterface) CommitsBetweenWithMeta(_ Range) ([]Commit, error) {
	return m.MockCommitsBetweenWithMeta, nil
}

func (m MockInterface) HeadTagOrCommit() (string, error) {
	return m.MockHeadOrTagCommit, nil
}

func (m MockInterface) HeadTag() (string, error) {
	return m.MockHeadTag, nil
}

func (m MockInterface) RemoteURL() (string, error) {
	return m.MockRemoteURL, nil
}

func (m MockInterface) SearchForTag(_ string) (*Tag, error) {
	if m.MockSearchTag == "" {
		return nil, nil
	}
	return &Tag{Name: m.MockSearchTag}, nil
}

func (m MockInterface) TagsFromLocal() ([]Tag, error) {
	var tags []Tag
	for _, t := range m.MockTags {
		tags = append(tags, Tag{
			Name: t,
		})
	}
	return tags, nil
}

func (m MockInterface) FirstCommit() (string, error) {
	return m.MockFirstCommit, nil
}

func (m MockInterface) WorktreeDirtyPaths() ([]string, error) {
	return m.MockDirtyPaths, nil
}

func (m MockInterface) ListFilesAtRef(ref string, match func(path string) bool) ([]FileBlob, error) {
	var out []FileBlob
	for _, f := range m.MockFilesAtRef[ref] {
		if match == nil || match(f.Path) {
			out = append(out, f)
		}
	}
	return out, nil
}
