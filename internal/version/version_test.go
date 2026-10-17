package version_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/version"
)

func TestString_Default(t *testing.T) {
	// The package defaults must be usable without a stamped build.
	if version.Version == "" {
		t.Fatal("Version is empty")
	}
	got := version.String()
	if got != version.Version {
		t.Fatalf("String() = %q, want %q (no commit stamped)", got, version.Version)
	}
}

func TestString_WithCommit(t *testing.T) {
	oldVersion, oldCommit := version.Version, version.Commit
	t.Cleanup(func() { version.Version, version.Commit = oldVersion, oldCommit })

	version.Version, version.Commit = "1.2.3", "abc1234"
	if got, want := version.String(), "1.2.3 (abc1234)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(version.String(), "1.2.3") {
		t.Fatalf("String() = %q, want a 1.2.3 prefix", version.String())
	}
}
