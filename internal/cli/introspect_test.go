package cli_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/cli"
)

func TestRule_ListsEveryCatalog(t *testing.T) {
	code, stdout, _ := run(t, "", "rule")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 83 {
		t.Fatalf("listed %d rules, want 83", len(lines))
	}
	for _, want := range []string{"X001", "O004", "QW001", "XP001", "VW001", "SEC001", "ST001", "RUX103"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("listing is missing %s", want)
		}
	}
}

func TestRule_Detail(t *testing.T) {
	code, stdout, _ := run(t, "", "rule", "O004")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"O004", "record-missing-model", "Odoo", "correctness", "enabled", "none"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("detail missing %q:\n%s", want, stdout)
		}
	}
}

func TestRule_DetailShowsFix(t *testing.T) {
	_, stdout, _ := run(t, "", "rule", "X014")
	if !strings.Contains(stdout, "unsafe") {
		t.Fatalf("X014 should report an unsafe fix:\n%s", stdout)
	}
	_, stdout, _ = run(t, "", "rule", "ST002")
	if !strings.Contains(stdout, "safe") {
		t.Fatalf("ST002 should report a safe fix:\n%s", stdout)
	}
}

func TestRule_Selector(t *testing.T) {
	code, stdout, _ := run(t, "", "rule", "XP0")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, "selects 9 rules") {
		t.Fatalf("XP0 should select 9 rules:\n%s", stdout)
	}
	if strings.Contains(stdout, "XP010") {
		t.Fatalf("XP0 must not include XP010:\n%s", stdout)
	}
}

func TestRule_UnknownCode(t *testing.T) {
	code, _, stderr := run(t, "", "rule", "ZZ999")
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "ZZ999") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestConfig_ListsKeysAndDefaults(t *testing.T) {
	code, stdout, _ := run(t, "", "config")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"line-length", "88", "indent-width", "4", "target-version", "auto", "respect-gitignore", "true", "lint.select", "correctness", "odoo.modules-path"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("config listing missing %q:\n%s", want, stdout)
		}
	}
}

func TestConfig_SingleKey(t *testing.T) {
	code, stdout, _ := run(t, "", "config", "line-length")
	if code != cli.ExitClean {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, "line-length = 88") {
		t.Fatalf("config line-length = %q", stdout)
	}
}

func TestConfig_UnknownKey(t *testing.T) {
	code, _, stderr := run(t, "", "config", "nope")
	if code != cli.ExitAbnormal {
		t.Fatalf("exit = %d, want %d", code, cli.ExitAbnormal)
	}
	if !strings.Contains(stderr, "nope") {
		t.Fatalf("stderr = %q", stderr)
	}
}
