package change

import (
	"strings"

	conventionalcommits "github.com/leodido/go-conventionalcommits"
	ccparser "github.com/leodido/go-conventionalcommits/parser"
)

// BreakingChangePrefix is the sentinel "prefix" used to map the conventional-commit
// breaking-change marker ("!") to a change type, e.g. "feat!: ...".
const BreakingChangePrefix = "!"

// parseConventionalCommit parses a subject line as a conventional commit using
// the given type set. ok is false when the subject is not a conventional commit.
func parseConventionalCommit(title string, types conventionalcommits.TypeConfig) (*conventionalcommits.ConventionalCommit, bool) {
	msg, err := ccparser.NewMachine(ccparser.WithTypes(types)).Parse([]byte(title))
	if err != nil || msg == nil || !msg.Ok() {
		return nil, false
	}
	cc, ok := msg.(*conventionalcommits.ConventionalCommit)
	if !ok {
		return nil, false
	}
	return cc, true
}

// ParseConventionalCommit reports the conventional-commit type (e.g. "feat",
// "fix") and whether the subject marks a breaking change. ok is false when the
// subject is not a conventional commit. A PR title carries no body or footer, so
// the trailing "!" before the colon is the only breaking signal we can observe.
//
// Free-form types are used so arbitrary (user-configured) prefixes parse rather
// than being gated by the library's built-in conventional type whitelist;
// callers decide which prefixes are meaningful via their own configured mapping.
func ParseConventionalCommit(title string) (ccType string, breaking bool, ok bool) {
	cc, ok := parseConventionalCommit(title, conventionalcommits.TypesFreeForm)
	if !ok {
		return "", false, false
	}
	return cc.Type, cc.IsBreakingChange(), true
}

// TrimConventionalCommitPrefix removes a leading conventional-commit prefix
// (e.g. "feat: ", "fix(scope)!: ") from a subject line, returning the bare
// description. The standard conventional-commit types are always recognized;
// recognizedTypes additionally allows caller-configured (non-standard) type
// prefixes — the same ones used to drive categorization — to be trimmed, so the
// display text stays consistent with how the subject was classified. An
// arbitrary "word: ..." subject with an unrecognized prefix is returned
// unchanged.
func TrimConventionalCommitPrefix(title string, recognizedTypes ...string) string {
	// standard conventional types are always trimmed.
	if _, ok := parseConventionalCommit(title, conventionalcommits.TypesConventional); ok {
		return trimAfterColon(title)
	}
	// otherwise only trim when the (free-form) type prefix is one the caller
	// recognizes. Free-form parsing lets any "word: ..." prefix parse, so the
	// recognized set gates which ones we actually strip — unrelated subjects like
	// "feature: ..." are left intact.
	if cc, ok := parseConventionalCommit(title, conventionalcommits.TypesFreeForm); ok && containsFold(recognizedTypes, cc.Type) {
		return trimAfterColon(title)
	}
	return title
}

// trimAfterColon returns the subject text following the first ":", trimmed of
// surrounding whitespace. We split rather than use the parsed description to
// preserve the original (untrimmed-by-the-grammar) message verbatim.
func trimAfterColon(title string) string {
	if fields := strings.SplitN(title, ":", 2); len(fields) == 2 {
		return strings.TrimSpace(fields[1])
	}
	return title
}

// containsFold reports whether needle is in haystack, compared case-insensitively.
func containsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}
