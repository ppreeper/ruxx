package attrorder_test

import (
	"strings"
	"testing"

	"github.com/ppreeper/ruxx/internal/attrorder"
)

// apply returns names reordered by Order, for readable assertions.
func apply(elem string, names []string) []string {
	out := make([]string, 0, len(names))
	for _, i := range attrorder.Order(elem, names) {
		out = append(out, names[i])
	}
	return out
}

func TestOrder_Alphabetical(t *testing.T) {
	got := strings.Join(apply("field", []string{"widget", "name", "readonly"}), " ")
	if got != "name readonly widget" {
		t.Errorf("got %q, want %q", got, "name readonly widget")
	}
}

func TestOrder_RecordIDBeforeModel(t *testing.T) {
	got := strings.Join(apply("record", []string{"model", "id", "context"}), " ")
	if got != "id model context" {
		t.Errorf("got %q, want %q", got, "id model context")
	}
}

func TestOrder_XPathExprBeforePosition(t *testing.T) {
	got := strings.Join(apply("xpath", []string{"position", "name", "expr"}), " ")
	if got != "expr position name" {
		t.Errorf("got %q, want %q", got, "expr position name")
	}
}

func TestOrder_AttributeNameFirst(t *testing.T) {
	got := strings.Join(apply("attribute", []string{"separator", "add", "name"}), " ")
	if got != "name add separator" {
		t.Errorf("got %q, want %q", got, "name add separator")
	}
}

func TestOrder_QWebDirectivesKeepSourceOrder(t *testing.T) {
	// t-* attributes come first and are not sorted among themselves.
	got := strings.Join(apply("div", []string{"class", "t-if", "t-foreach", "id"}), " ")
	if got != "t-if t-foreach class id" {
		t.Errorf("got %q, want %q", got, "t-if t-foreach class id")
	}
}

func TestOrder_QWebBeforePriority(t *testing.T) {
	got := strings.Join(apply("record", []string{"model", "t-if", "id"}), " ")
	if got != "t-if id model" {
		t.Errorf("got %q, want %q", got, "t-if id model")
	}
}

func TestOrder_MenuitemIDBeforeName(t *testing.T) {
	got := strings.Join(apply("menuitem", []string{"sequence", "name", "id"}), " ")
	if got != "id name sequence" {
		t.Errorf("got %q, want %q", got, "id name sequence")
	}
}

func TestOrder_FieldNameFirst(t *testing.T) {
	got := strings.Join(apply("field", []string{"widget", "type", "name"}), " ")
	if got != "name type widget" {
		t.Errorf("got %q, want %q", got, "name type widget")
	}
}

func TestOrder_StableTies(t *testing.T) {
	// No duplicates in XML, but stability must not depend on the comparator.
	got := strings.Join(apply("field", []string{"name", "name"}), " ")
	if got != "name name" {
		t.Errorf("got %q, want %q", got, "name name")
	}
}

func TestSorted(t *testing.T) {
	cases := []struct {
		elem  string
		names []string
		want  bool
	}{
		{"field", []string{"name", "widget"}, true},
		{"field", []string{"widget", "name"}, false},
		{"record", []string{"id", "model"}, true},
		{"record", []string{"model", "id"}, false},
		{"div", []string{"t-if", "class"}, true},
		{"div", []string{"class", "t-if"}, false},
		{"field", nil, true},
		{"field", []string{"name"}, true},
	}
	for _, c := range cases {
		if got := attrorder.Sorted(c.elem, c.names); got != c.want {
			t.Errorf("Sorted(%q, %v) = %v, want %v", c.elem, c.names, got, c.want)
		}
	}
}

func TestPriority(t *testing.T) {
	if got := attrorder.Priority("record"); len(got) != 2 || got[0] != "id" || got[1] != "model" {
		t.Errorf("Priority(record) = %v", got)
	}
	if got := attrorder.Priority("field"); len(got) != 1 || got[0] != "name" {
		t.Errorf("Priority(field) = %v, want [name]", got)
	}
	if got := attrorder.Priority("div"); got != nil {
		t.Errorf("Priority(div) = %v, want nil", got)
	}
}
