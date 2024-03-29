package git

var _ Interface = (*MockInterface)(nil)

type MockInterface struct {
	MockHeadOrTagCommit string
	MockHeadTag         string
	MockTags            []string
	MockRemoteURL       string
	MockSearchTag       string
	MockCommitsBetween  []string
	MockFirstCommit     string
}

func (m MockInterface) CommitsBetween(_ Range) ([]string, error) {
	return m.MockCommitsBetween, nil
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
