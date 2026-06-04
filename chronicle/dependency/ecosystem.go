package dependency

import "strings"

// Ecosystem is chronicle's canonical identifier for a language ecosystem (e.g.
// Go, JavaScript). It is the typed vocabulary chronicle reasons about for
// toolchain detection and display grouping. It is deliberately distinct from two
// adjacent syft-owned string vocabularies that stay as raw strings at the syft
// boundary:
//
//   - Package.Type — the raw syft package type (e.g. "go-module", "deb"). Open
//     ended and includes OS/infra families with no ecosystem analog; parsed into
//     an Ecosystem only for presentation (with a raw fallback so nothing drops).
//   - dependency selectors — the syft cataloger selection expressions a user
//     passes to --dependencies (e.g. "language", "go", or richer cataloger/tag
//     expressions), which an enum cannot fully represent.
//
// The zero value EcosystemUnknown represents an unrecognized input.
type Ecosystem string

const (
	EcosystemUnknown    Ecosystem = ""
	EcosystemGo         Ecosystem = "go"
	EcosystemJavaScript Ecosystem = "javascript"
	EcosystemPython     Ecosystem = "python"
	EcosystemJava       Ecosystem = "java"
	EcosystemRuby       Ecosystem = "ruby"
	EcosystemRust       Ecosystem = "rust"
	EcosystemPHP        Ecosystem = "php"
	EcosystemDotNet     Ecosystem = "dotnet"
	EcosystemCPP        Ecosystem = "cpp"
	EcosystemSwift      Ecosystem = "swift"
	EcosystemDart       Ecosystem = "dart"
	EcosystemHaskell    Ecosystem = "haskell"
	EcosystemElixir     Ecosystem = "elixir"
	EcosystemCocoaPods  Ecosystem = "cocoapods"
)

// ecosystemInfo carries the presentation label, the canonical syft cataloger
// selector, and the recognized aliases for one ecosystem. Aliases include the
// syft package type(s) and common selector/name spellings so a single table is
// the source of truth for both parsing directions.
type ecosystemInfo struct {
	eco      Ecosystem
	label    string   // display label, e.g. "JavaScript", ".NET", "C/C++"
	selector string   // canonical syft cataloger selection expression, e.g. "javascript"
	aliases  []string // syft package types + selector/name spellings (lowercase)
}

// ecosystemTable is the canonical, ordered registry of known ecosystems. Order
// is the display/sort order (common languages first); it is the single source
// for Ecosystems(), Label, Selector, and Parse.
var ecosystemTable = []ecosystemInfo{
	{EcosystemGo, "Go", "go", []string{"go", "golang", "go-module"}},
	{EcosystemJavaScript, "JavaScript", "javascript", []string{"javascript", "js", "node", "nodejs", "npm", "yarn", "pnpm"}},
	{EcosystemPython, "Python", "python", []string{"python", "py", "pip", "wheel", "egg", "poetry"}},
	{EcosystemJava, "Java", "java", []string{"java", "java-archive", "jenkins-plugin", "graalvm-native-image", "maven", "gradle"}},
	{EcosystemRuby, "Ruby", "ruby", []string{"ruby", "gem", "gemspec"}},
	{EcosystemRust, "Rust", "rust", []string{"rust", "rust-crate", "cargo"}},
	{EcosystemPHP, "PHP", "php", []string{"php", "php-composer", "composer"}},
	{EcosystemDotNet, ".NET", "dotnet", []string{"dotnet", "nuget", "csharp"}},
	{EcosystemCPP, "C/C++", "cpp", []string{"cpp", "c", "c++", "conan"}},
	{EcosystemSwift, "Swift", "swift", []string{"swift", "swift-package-manager"}},
	{EcosystemDart, "Dart", "dart", []string{"dart", "dart-pub", "pub"}},
	{EcosystemHaskell, "Haskell", "haskell", []string{"haskell", "hackage"}},
	{EcosystemElixir, "Erlang/Elixir", "elixir", []string{"elixir", "erlang", "hex", "erlang-otp"}},
	{EcosystemCocoaPods, "CocoaPods", "cocoapods", []string{"cocoapods", "cocoapod", "pod"}},
}

// aliasIndex maps every recognized alias (and canonical value) to its Ecosystem.
var aliasIndex = func() map[string]Ecosystem {
	m := make(map[string]Ecosystem)
	for _, info := range ecosystemTable {
		m[string(info.eco)] = info.eco
		for _, a := range info.aliases {
			m[a] = info.eco
		}
	}
	return m
}()

// infoByEcosystem indexes the table for O(1) Label/Selector lookups.
var infoByEcosystem = func() map[Ecosystem]ecosystemInfo {
	m := make(map[Ecosystem]ecosystemInfo, len(ecosystemTable))
	for _, info := range ecosystemTable {
		m[info.eco] = info
	}
	return m
}()

// ParseEcosystem maps a raw syft package type or cataloger selector to its
// canonical Ecosystem. Matching is case-insensitive and trims surrounding
// whitespace. ok is false for an unrecognized value (callers decide the
// fallback, e.g. grouping by the raw string so nothing is dropped).
func ParseEcosystem(s string) (Ecosystem, bool) {
	e, ok := aliasIndex[strings.ToLower(strings.TrimSpace(s))]
	return e, ok
}

// Ecosystems returns the known ecosystems in canonical display order.
func Ecosystems() []Ecosystem {
	out := make([]Ecosystem, 0, len(ecosystemTable))
	for _, info := range ecosystemTable {
		out = append(out, info.eco)
	}
	return out
}

// String returns the canonical lowercase identifier (the enum value).
func (e Ecosystem) String() string { return string(e) }

// Label returns the human-friendly display label (e.g. "JavaScript", ".NET").
// An unknown ecosystem falls back to its raw value so callers never render an
// empty group title.
func (e Ecosystem) Label() string {
	if info, ok := infoByEcosystem[e]; ok {
		return info.label
	}
	return string(e)
}

// Selector returns the canonical syft cataloger selection expression for the
// ecosystem (e.g. "javascript"), or the raw value when unknown.
func (e Ecosystem) Selector() string {
	if info, ok := infoByEcosystem[e]; ok {
		return info.selector
	}
	return string(e)
}
