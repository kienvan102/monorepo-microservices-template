package jsrunner

import (
	"strings"
	"testing"

	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

func TestBuildProgram(t *testing.T) {
	cfg := settings.MongoConfig{Collection: "orders", URI: "mongodb://host", Database: "shop"}

	t.Run("a plain script is options, connection, source", func(t *testing.T) {
		program, err := buildProgram(Resolved{Source: "run();"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		text := string(program)
		if strings.Count(text, "globalThis.") != 1 || !strings.Contains(text, "globalThis."+optionsGlobal+" = ") {
			t.Errorf("only the options may be set:\n%s", text)
		}
		if !strings.HasSuffix(text, "run();\n") {
			t.Errorf("source must come last:\n%s", text)
		}
	})
	t.Run("items come in the group's variable, in order, before the source", func(t *testing.T) {
		resolved := Resolved{
			Source: "run();",
			Global: "ORDER_QUERIES",
			Items: []Item{
				{Name: "slow", Source: "// note\n() => ({ a: 1 })"},
				{Name: "recent", Source: "() => ({ b: 2 })"},
			},
		}
		program, err := buildProgram(resolved, cfg)
		if err != nil {
			t.Fatal(err)
		}
		text := string(program)
		global := strings.Index(text, "globalThis.ORDER_QUERIES = [")
		slow, recent, source := strings.Index(text, `name: "slow"`), strings.Index(text, `name: "recent"`), strings.Index(text, "run();")
		if global < 0 || slow < global || recent < slow || source < recent {
			t.Errorf("wrong order (global=%d slow=%d recent=%d source=%d):\n%s", global, slow, recent, source, text)
		}
		if !strings.Contains(text, "build: (\n// note\n() => ({ a: 1 })\n) },") {
			t.Errorf("an item file must be wrapped as one expression:\n%s", text)
		}
	})
	t.Run("an engine with no planned items still gets its variable", func(t *testing.T) {
		program, err := buildProgram(Resolved{Source: "run();", Global: "ORDER_QUERIES"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(program), "globalThis.ORDER_QUERIES = [\n];\n") {
			t.Errorf("want an empty array:\n%s", program)
		}
	})
}
