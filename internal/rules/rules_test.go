package rules_test

import (
	"testing"

	"github.com/ppreeper/ruxx/internal/linttest"
)

// TestRules runs every golden case under testdata/rules via the harness.
// Each case selects exactly the rule named by its parent directory, so the
// expected.txt files are the contract the rule implementations must satisfy.
func TestRules(t *testing.T) {
	linttest.RunAll(t, "testdata/rules")
}
