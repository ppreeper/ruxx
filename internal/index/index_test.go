package index_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ppreeper/ruxx/internal/index"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuild_XMLIDsAndManifest(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "mymodule")
	write(t, mod, "__manifest__.py", `{
    'name': 'My Module',
    'data': [
        'security/ir.model.access.csv',
        'views/my_views.xml',
    ],
}`)
	write(t, filepath.Join(mod, "views"), "my_views.xml", `<?xml version="1.0"?>
<odoo>
    <record id="view_a" model="ir.ui.view"/>
    <record id="mymodule.view_b" model="ir.ui.view"/>
    <record id="grp" model="res.groups"/>
</odoo>`)
	write(t, filepath.Join(mod, "security"), "ir.model.access.csv", "id,name,model_id:id\n")

	ix, err := index.Build([]string{root})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, id := range []string{"mymodule.view_a", "mymodule.view_b", "mymodule.grp"} {
		if !ix.HasXMLID(id) {
			t.Errorf("HasXMLID(%q) = false, want true", id)
		}
	}
	if ix.HasXMLID("mymodule.missing") {
		t.Error("HasXMLID(mymodule.missing) = true, want false")
	}
	m, ok := ix.Module("mymodule")
	if !ok {
		t.Fatal("module mymodule not indexed")
	}
	if !m.HasManifest {
		t.Error("HasManifest = false, want true")
	}
	if len(m.Data) != 2 || m.Data[0] != "security/ir.model.access.csv" || m.Data[1] != "views/my_views.xml" {
		t.Errorf("Data = %v", m.Data)
	}
	if !m.Groups["mymodule.grp"] {
		t.Errorf("Groups = %v, want mymodule.grp", m.Groups)
	}
	if m.AccessCSV == "" {
		t.Error("AccessCSV empty, want path")
	}
}

func TestBuild_DuplicateXMLID(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "dupmod")
	write(t, mod, "__manifest__.py", `{'data': ['views/a.xml', 'views/b.xml']}`)
	write(t, filepath.Join(mod, "views"), "a.xml", `<odoo><record id="x" model="ir.ui.view"/></odoo>`)
	write(t, filepath.Join(mod, "views"), "b.xml", `<odoo><record id="x" model="ir.ui.view"/></odoo>`)

	ix, err := index.Build([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	m, _ := ix.Module("dupmod")
	if got := len(m.Defs["dupmod.x"]); got != 2 {
		t.Errorf("Defs[dupmod.x] has %d files, want 2", got)
	}
}

func TestBuild_TNameTemplate(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "webmod")
	write(t, filepath.Join(mod, "static", "src", "xml"), "tmpl.xml",
		`<templates xml:space="preserve"><t t-name="webmod.Widget">hi</t></templates>`)

	ix, err := index.Build([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if !ix.HasXMLID("webmod.Widget") {
		t.Error("t-name template not indexed")
	}
}

func TestBuild_MissingRootSkipped(t *testing.T) {
	ix, err := index.Build([]string{filepath.Join(t.TempDir(), "nope")})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(ix.Modules()) != 0 {
		t.Errorf("Modules = %d, want 0", len(ix.Modules()))
	}
}

func TestBuild_MultipleRoots(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write(t, filepath.Join(a, "mod_a", "views"), "a.xml", `<odoo><record id="r" model="ir.ui.view"/></odoo>`)
	write(t, filepath.Join(b, "mod_b", "views"), "b.xml", `<odoo><record id="r" model="ir.ui.view"/></odoo>`)

	ix, err := index.Build([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !ix.HasModule("mod_a") || !ix.HasModule("mod_b") {
		t.Errorf("modules = %v", ix.Modules())
	}
	if !ix.HasXMLID("mod_a.r") || !ix.HasXMLID("mod_b.r") {
		t.Error("xmlids missing from multi-root build")
	}
}
