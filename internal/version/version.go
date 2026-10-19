// Package version holds ruxx's build identity.
//
// The values are variables rather than constants so a release build can stamp
// them:
//
//	go build -ldflags "-X github.com/ppreeper/ruxx/internal/version.Version=1.2.3 \
//	    -X github.com/ppreeper/ruxx/internal/version.Commit=abc1234 \
//	    -X github.com/ppreeper/ruxx/internal/version.Date=2026-01-02T15:04:05Z" ./cmd/ruxx
//
// `make build`, `make install`, and the goreleaser pipeline do this
// automatically; a plain `go build` keeps the development defaults.
package version

// Version is the released version, without a leading "v" (e.g. "1.2.3").
var Version = "0.1.1-dev"

// Commit is the short commit the binary was built from, or "" when unknown.
var Commit = ""

// Date is the build timestamp in RFC 3339 (UTC), or "" when unknown.
var Date = ""

// String returns the version, with the commit in parentheses when the build
// stamped one.
func String() string {
	if Commit == "" {
		return Version
	}
	return Version + " (" + Commit + ")"
}
