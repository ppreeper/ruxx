package cli_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

// TestConfig_ListsFormatKeys locks the [format] surface into `ruxx config`.
func TestConfig_ListsFormatKeys(t *testing.T) {
	code, stdout, _ := run(t, "", "config")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want %d", code, cli.ExitClean)
	}
	for _, key := range []string{
		"format.attribute-split",
		"format.split-attributes-tags",
		"format.close-tag-on-new-line",
		"format.empty-element",
		"format.odoo-tag-spacing",
		"format.odoo-spacing-tags",
		"format.attribute-sort",
		"format.indent-style",
		"format.line-ending",
		"format.final-newline",
		"format.xml-declaration",
		"format.encoding-style",
		"format.qweb-attributes-first",
	} {
		if !strings.Contains(stdout, key) {
			t.Errorf("ruxx config does not list %s", key)
		}
	}
}
