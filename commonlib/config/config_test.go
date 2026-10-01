package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnvName(t *testing.T) {
	cases := map[string]string{
		"uri":           "URI",
		"queryTimeout":  "QUERY_TIMEOUT",
		"query_timeout": "QUERY_TIMEOUT",
		"query-timeout": "QUERY_TIMEOUT",
		"mongoURI":      "MONGO_U_R_I",
		"MongoUri":      "MONGO_URI",
		"mongo2":        "MONGO2",
		"appEnv":        "APP_ENV",
	}
	for key, want := range cases {
		if got := envName(key); got != want {
			t.Errorf("envName(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestFlattenYAML(t *testing.T) {
	got, err := flattenYAML([]byte(`
appEnv: dev
mongo:
  uri: mongodb://localhost:27017
  database: shop
queryTimeout: 20s
sampleSize: 0500
enabled: true
empty: ""
unset:
hosts: [a, b, c]
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"APP_ENV":        "dev",
		"MONGO_URI":      "mongodb://localhost:27017",
		"MONGO_DATABASE": "shop",
		"QUERY_TIMEOUT":  "20s",
		"SAMPLE_SIZE":    "0500",
		"ENABLED":        "true",
		"EMPTY":          "",
		"HOSTS":          "a,b,c",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestFlattenYAMLErrors(t *testing.T) {
	cases := map[string]string{
		"duplicate name":  "mongoUri: a\nmongo:\n  uri: b\n",
		"list of mapping": "hosts:\n  - name: a\n",
		"not a mapping":   "- a\n- b\n",
	}
	for name, input := range cases {
		if _, err := flattenYAML([]byte(input)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

type layered struct {
	Env      string `env:"CFGTEST_ENV" envDefault:"default"`
	EnvFile  string `env:"CFGTEST_ENV_FILE" envDefault:"default"`
	YAML     string `env:"CFGTEST_YAML" envDefault:"default"`
	Defaults string `env:"CFGTEST_DEFAULTS" envDefault:"default"`
}

func TestLoadPriority(t *testing.T) {
	dir := t.TempDir()
	yamlPath := write(t, dir, "config.yaml", "cfgtest:\n  env: yaml\n  envFile: yaml\n  yaml: yaml\n")
	envPath := write(t, dir, ".env", "CFGTEST_ENV=file\nCFGTEST_ENV_FILE=file\n")
	t.Setenv("CFGTEST_ENV", "real")

	got, err := Load[layered](yamlPath, envPath)
	if err != nil {
		t.Fatal(err)
	}
	want := layered{Env: "real", EnvFile: "file", YAML: "yaml", Defaults: "default"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadMissingFiles(t *testing.T) {
	dir := t.TempDir()
	yamlPath := write(t, dir, "config.yaml", "cfgtest:\n  yaml: yaml\n")

	got, err := Load[layered](yamlPath, filepath.Join(dir, "missing.env"))
	if err != nil {
		t.Fatalf("missing .env must be skipped, got %v", err)
	}
	if got.YAML != "yaml" {
		t.Errorf("YAML = %q, want %q", got.YAML, "yaml")
	}

	_, err = Load[layered](filepath.Join(dir, "missing.yaml"), "")
	if err == nil || !strings.Contains(err.Error(), "missing.yaml") {
		t.Errorf("missing YAML must be an error naming the file, got %v", err)
	}
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type prefixed struct {
	URI  string `env:"CFGTEST_URI" envDefault:"default"`
	Name string `env:"CFGTEST_NAME" envDefault:"default"`
}

func TestLoadPrefix(t *testing.T) {
	dir := t.TempDir()
	yamlPath := write(t, dir, "config.yaml", "cfgtest:\n  uri: plain\nanalyzer:\n  cfgtest:\n    uri: from-yaml\n")
	t.Setenv("ANALYZER_CFGTEST_NAME", "from-env")

	got, err := Load[prefixed](yamlPath, "", WithPrefix("analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	want := prefixed{URI: "from-yaml", Name: "from-env"}
	if got != want {
		t.Errorf("with prefix: got %+v, want %+v", got, want)
	}

	plain, err := Load[prefixed](yamlPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if plain.URI != "plain" {
		t.Errorf("without prefix: URI = %q, want %q", plain.URI, "plain")
	}
}

type withPath struct {
	Relative Path `env:"CFGTEST_REL"`
	Absolute Path `env:"CFGTEST_ABS"`
	Default  Path `env:"CFGTEST_DEFAULT_PATH" envDefault:"output"`
	Empty    Path `env:"CFGTEST_EMPTY_PATH"`
}

func TestLoadPath(t *testing.T) {
	dir := t.TempDir()
	yamlPath := write(t, dir, "config.yaml", "cfgtest:\n  rel: ../out\n  abs: /var/out\n")
	envPath := filepath.Join(dir, "env", ".env")

	got, err := Load[withPath](yamlPath, envPath)
	if err != nil {
		t.Fatal(err)
	}
	want := withPath{
		Relative: Path(filepath.Join(dir, "../out")),
		Absolute: "/var/out",
		Default:  Path(filepath.Join(dir, "output")),
	}
	if got != want {
		t.Errorf("relative to YAML dir: got %+v\nwant %+v", got, want)
	}

	t.Setenv("CFGTEST_REL", "out")
	got, err = Load[withPath]("", envPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := Path(filepath.Join(dir, "env", "out")); got.Relative != want {
		t.Errorf("relative to .env dir: got %q, want %q", got.Relative, want)
	}

	got, err = Load[withPath]("", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Relative != "out" {
		t.Errorf("no files: got %q, want it unchanged", got.Relative)
	}
}
