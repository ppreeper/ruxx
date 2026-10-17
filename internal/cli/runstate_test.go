package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The run state (resolved config, lint cache, cross-file index) is shared by the
// parallel workers. These tests exercise it through cli.Run: the index must be
// built and used, and each file must still get its own directory's config.

const viewSrc = `<?xml version="1.0" encoding="utf-8"?>
<odoo>
    <record id="%s" model="ir.ui.view">
        <field name="name">v</field>
        <field name="model">res.partner</field>
        <field name="arch" type="xml"><form/></field>
    </record>
</odoo>
`

// writeIn writes a file, creating its directory first.
func writeIn(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return writeFile(t, dir, name, content)
}

// indexProject writes a project of nModules × 2 view files and returns its root.
// The module "mod_00" defines the same xmlid in both of its files, which O018
// reports; every other module is clean.
func indexProject(t *testing.T, nModules int) string {
	t.Helper()
	root := t.TempDir()
	addons := filepath.Join(root, "addons")
	writeIn(t, root, ".ruxx.toml", "[odoo]\nmodules-path = [\"addons\"]\n")
	for m := 0; m < nModules; m++ {
		mod := fmt.Sprintf("mod_%02d", m)
		dir := filepath.Join(addons, mod)
		writeIn(t, dir, "__manifest__.py",
			fmt.Sprintf("{\n    \"name\": %q,\n    \"data\": [\"views/a.xml\", \"views/b.xml\"],\n}\n", mod))
		idA, idB := "a_"+mod, "b_"+mod
		if m == 0 {
			// mod_00 defines the same xmlid in both files; the rest are distinct.
			idA, idB = "same", "same"
		}
		writeIn(t, filepath.Join(dir, "views"), "a.xml", fmt.Sprintf(viewSrc, idA))
		writeIn(t, filepath.Join(dir, "views"), "b.xml", fmt.Sprintf(viewSrc, idB))
	}
	return root
}

func TestCheck_IndexSharedAcrossWorkers(t *testing.T) {
	root := indexProject(t, 6)
	args := []string{"check", "-n", "--select", "O018", filepath.Join(root, "addons")}
	code, stdout, stderr := run(t, "", args...)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", code, stderr)
	}
	// Both files of mod_00 define "mod_00.same".
	if n := strings.Count(stdout, "O018"); n != 2 {
		t.Errorf("O018 diagnostics = %d, want 2:\n%s", n, stdout)
	}
	if !strings.Contains(stdout, `"mod_00.same" is defined in 2 files`) {
		t.Errorf("missing duplicate-xmlid message:\n%s", stdout)
	}
	// The shared index must not change the answer between runs.
	_, again, _ := run(t, "", args...)
	if again != stdout {
		t.Errorf("index results are not deterministic:\n%s\n---\n%s", stdout, again)
	}
}

func TestCheck_PerDirectoryConfig(t *testing.T) {
	root := t.TempDir()
	// The parent config reports record-missing-model; the nested one reports
	// trailing whitespace instead, so each file proves which config it used.
	writeIn(t, root, ".ruxx.toml", "[lint]\nselect = [\"O004\"]\n")
	writeIn(t, root, "a.xml", badO004)
	sub := filepath.Join(root, "sub")
	writeIn(t, sub, ".ruxx.toml", "[lint]\nselect = [\"X007\"]\n")
	writeIn(t, sub, "b.xml", "<?xml version=\"1.0\"?>\n<odoo>   \n</odoo>\n")

	code, stdout, stderr := run(t, "", "check", "-n", root)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", code, stderr)
	}
	codes := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		rest := line[i+1:]
		j := strings.Index(rest, ": ")
		if j < 0 {
			continue
		}
		base := filepath.Base(line[:i])
		codes[base] = append(codes[base], strings.Fields(rest[j+2:])[0])
	}
	if got := codes["a.xml"]; len(got) != 1 || got[0] != "O004" {
		t.Errorf("a.xml codes = %v, want [O004]\n%s", got, stdout)
	}
	if got := codes["b.xml"]; len(got) != 1 || got[0] != "X007" {
		t.Errorf("b.xml codes = %v, want [X007]\n%s", got, stdout)
	}
}
