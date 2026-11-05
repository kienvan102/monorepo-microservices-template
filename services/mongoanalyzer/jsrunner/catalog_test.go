package jsrunner

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// groupFS has one collection with a group (orders) and one without (books).
// orders' "queries" group has two engines, three item files of which two are
// planned, and a folder (stray) that no plan.json declares.
func groupFS() fstest.MapFS {
	return fstest.MapFS{
		"plan.json":       {Data: []byte(`{"inspect": ["common/stats"]}`)},
		"common/stats.js": {Data: []byte("// common")},

		"collections/orders/plan.json": {Data: []byte(`{
			"default": ["profile", "explain"],
			"groups": {
				"queries": {"engines": ["explain", "compare"], "global": "ORDER_QUERIES", "items": ["slow", "recent"]}
			}
		}`)},
		"collections/orders/profile.js":          {Data: []byte("// plain script")},
		"collections/orders/explain.js":          {Data: []byte("// explain engine")},
		"collections/orders/compare.js":          {Data: []byte("// compare engine")},
		"collections/orders/queries/slow.js":     {Data: []byte("() => ({ filter: {} })")},
		"collections/orders/queries/recent.js":   {Data: []byte("() => ({ pipeline: [] })")},
		"collections/orders/queries/unlisted.js": {Data: []byte("() => ({ filter: { a: 1 } })")},
		"collections/orders/stray/thing.js":      {Data: []byte("// folder no plan declares")},

		"collections/books/plan.json":  {Data: []byte(`{"default": ["profile"]}`)},
		"collections/books/profile.js": {Data: []byte("// plain script")},
	}
}

func TestGroupsAreReadFromPlan(t *testing.T) {
	groups, err := NewCatalog(groupFS()).groups("orders")
	if err != nil {
		t.Fatal(err)
	}
	want := Group{
		Name:    "queries",
		Engines: []string{"collections/orders/explain", "collections/orders/compare"},
		Global:  "ORDER_QUERIES",
		Items:   []string{"slow", "recent"},
	}
	if len(groups) != 1 || !reflect.DeepEqual(groups["queries"], want) {
		t.Errorf("groups = %+v, want only %+v", groups, want)
	}
}

func TestGroupsAreOptional(t *testing.T) {
	c := NewCatalog(groupFS())
	for _, collection := range []string{"books", "no-such-collection"} {
		got, err := c.groups(collection)
		if err != nil || len(got) != 0 {
			t.Errorf("groups(%q) = %v, %v; want none", collection, got, err)
		}
	}
}

func TestGroupDeclarationsAreChecked(t *testing.T) {
	cases := map[string]string{
		"bad json":           `{"groups": `,
		"no engines":         `{"groups": {"queries": {"engines": [], "global": "Q", "items": []}}}`,
		"engine not found":   `{"groups": {"queries": {"engines": ["ghost"], "global": "Q", "items": []}}}`,
		"engine path escape": `{"groups": {"queries": {"engines": ["../profile"], "global": "Q", "items": []}}}`,
		"item not found":     `{"groups": {"queries": {"engines": ["explain"], "global": "Q", "items": ["ghost"]}}}`,
		"item path escape":   `{"groups": {"queries": {"engines": ["explain"], "global": "Q", "items": ["../profile"]}}}`,
		"global not a name":  `{"groups": {"queries": {"engines": ["explain"], "global": "a b", "items": []}}}`,
		"global missing":     `{"groups": {"queries": {"engines": ["explain"], "items": []}}}`,
		"global is options":  `{"groups": {"queries": {"engines": ["explain"], "global": "MONGO_ANALYSIS_OPTIONS", "items": []}}}`,
		"group name escapes": `{"groups": {"../x": {"engines": ["explain"], "global": "Q", "items": []}}}`,
		"engine in two groups": `{"groups": {
			"a": {"engines": ["explain"], "global": "A", "items": []},
			"queries": {"engines": ["explain"], "global": "Q", "items": []}}}`,
	}
	for name, plan := range cases {
		t.Run(name, func(t *testing.T) {
			fsys := groupFS()
			fsys["collections/orders/plan.json"].Data = []byte(plan)
			_, err := NewCatalog(fsys).groups("orders")
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), "collections/orders/plan.json") && name != "bad json" {
				t.Errorf("error should name the file: %v", err)
			}
		})
	}
}

func TestItemsAreScriptsOfDeclaredGroupsOnly(t *testing.T) {
	c := NewCatalog(groupFS())

	got, err := c.Discover()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"collections/books/profile",
		"collections/orders/compare",
		"collections/orders/explain",
		"collections/orders/profile",
		"collections/orders/queries/recent",
		"collections/orders/queries/slow",
		"collections/orders/queries/unlisted",
		"common/stats",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Discover = %v, want %v", got, want)
	}

	for name, exists := range map[string]bool{
		"collections/orders/queries/slow":     true,
		"collections/orders/queries/unlisted": true, // not planned, still runnable by name
		"collections/orders/queries/ghost":    false,
		"collections/orders/stray/thing":      false, // folder no plan declares
		"collections/books/queries/slow":      false,
	} {
		if c.Exists(name) != exists {
			t.Errorf("Exists(%q) = %v, want %v", name, !exists, exists)
		}
	}

	if err := c.ValidateTarget("collections/orders/queries/slow", "orders"); err != nil {
		t.Errorf("ValidateTarget for the right collection: %v", err)
	}
	if err := c.ValidateTarget("collections/orders/queries/slow", "books"); err == nil {
		t.Error("ValidateTarget must reject another collection's item")
	}
}

func TestResolve(t *testing.T) {
	c := NewCatalog(groupFS())

	names := func(items []Item) []string {
		var got []string
		for _, item := range items {
			got = append(got, item.Name)
		}
		return got
	}

	t.Run("a plain script gets nothing handed to it", func(t *testing.T) {
		for _, name := range []string{"collections/orders/profile", "collections/books/profile", "common/stats"} {
			got, err := c.Resolve(name)
			if err != nil || got.Global != "" || got.Items != nil || got.Source == "" {
				t.Errorf("Resolve(%q) = %+v, %v", name, got, err)
			}
		}
	})
	t.Run("an engine gets the group's planned items in order", func(t *testing.T) {
		for _, name := range []string{"collections/orders/explain", "collections/orders/compare"} {
			got, err := c.Resolve(name)
			if err != nil {
				t.Fatal(err)
			}
			if got.Global != "ORDER_QUERIES" || !reflect.DeepEqual(names(got.Items), []string{"slow", "recent"}) {
				t.Errorf("Resolve(%q) = %+v", name, got)
			}
			if got.Items[0].Source != "() => ({ filter: {} })" {
				t.Errorf("item source = %q", got.Items[0].Source)
			}
		}
		got, _ := c.Resolve("collections/orders/compare")
		if got.Source != "// compare engine" {
			t.Errorf("must run its own source, got %q", got.Source)
		}
	})
	t.Run("an item name runs the default engine on that one item", func(t *testing.T) {
		got, err := c.Resolve("collections/orders/queries/unlisted")
		if err != nil {
			t.Fatal(err)
		}
		if got.Source != "// explain engine" || got.Global != "ORDER_QUERIES" || !reflect.DeepEqual(names(got.Items), []string{"unlisted"}) {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("an item of an undeclared folder is not run", func(t *testing.T) {
		if _, err := c.Resolve("collections/orders/stray/thing"); err == nil {
			t.Error("want an error")
		}
	})
	t.Run("a broken declaration is reported when its engine runs", func(t *testing.T) {
		fsys := groupFS()
		fsys["collections/orders/plan.json"].Data = []byte(`{"groups": {"queries": {"engines": ["ghost"], "global": "Q"}}}`)
		if _, err := NewCatalog(fsys).Resolve("collections/orders/explain"); err == nil || !strings.Contains(err.Error(), "collections/orders/plan.json") {
			t.Errorf("err = %v, want one naming the plan file", err)
		}
	})
}

// A new kind of script is a folder, an engine script and a plan.json entry:
// nothing in the catalog or the runner is edited.
func TestANewGroupNeedsNoGoChange(t *testing.T) {
	fsys := groupFS()
	fsys["collections/orders/plan.json"].Data = []byte(`{
		"default": ["profile", "explain"],
		"groups": {
			"queries": {"engines": ["explain"], "global": "ORDER_QUERIES", "items": ["slow"]},
			"checks":  {"engines": ["audit"], "global": "ORDER_CHECKS", "items": ["totals", "dupes"]}
		}
	}`)
	fsys["collections/orders/audit.js"] = &fstest.MapFile{Data: []byte("// audit engine")}
	fsys["collections/orders/checks/totals.js"] = &fstest.MapFile{Data: []byte("() => 'totals'")}
	fsys["collections/orders/checks/dupes.js"] = &fstest.MapFile{Data: []byte("() => 'dupes'")}
	c := NewCatalog(fsys)

	engine, err := c.Resolve("collections/orders/audit")
	if err != nil || engine.Global != "ORDER_CHECKS" || len(engine.Items) != 2 || engine.Items[0].Name != "totals" {
		t.Errorf("engine = %+v, %v", engine, err)
	}
	item, err := c.Resolve("collections/orders/checks/dupes")
	if err != nil || item.Source != "// audit engine" || item.Global != "ORDER_CHECKS" || len(item.Items) != 1 || item.Items[0].Name != "dupes" {
		t.Errorf("item = %+v, %v", item, err)
	}
	other, err := c.Resolve("collections/orders/explain")
	if err != nil || other.Global != "ORDER_QUERIES" || len(other.Items) != 1 {
		t.Errorf("the first group must be unaffected: %+v, %v", other, err)
	}
	if !c.Exists("collections/orders/checks/totals") {
		t.Error("the new group's items must be scripts")
	}
}

func TestPlanIgnoresGroupsAndKeepsScriptLists(t *testing.T) {
	c := NewCatalog(groupFS())

	got, err := c.Plan("default", "orders")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"common/stats", "collections/orders/profile", "collections/orders/explain"}
	if !slices.Equal(got, want) {
		t.Errorf("Plan(default) = %v, want %v", got, want)
	}

	got, err = c.Plan("inspect", "orders")
	if err != nil || !slices.Equal(got, []string{"common/stats"}) {
		t.Errorf("Plan(inspect) = %v, %v", got, err)
	}
}
