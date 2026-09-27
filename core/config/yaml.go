package config

import (
	"fmt"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// flattenYAML turns a YAML mapping into environment-style variables: nested
// keys are joined with "_" and each key is converted by envName, so
//
//	mongo:
//	  uri: x
//
// becomes MONGO_URI=x. Scalars keep the exact text written in the file.
func flattenYAML(data []byte) (map[string]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := map[string]string{}
	if len(root.Content) == 0 {
		return out, nil
	}
	doc := resolve(root.Content[0])
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("top level must be a mapping")
	}
	sources := map[string]string{}
	return out, flattenMapping(doc, "", "", out, sources)
}

func flattenMapping(node *yaml.Node, prefix, path string, out, sources map[string]string) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, resolve(node.Content[i+1])
		name, keyPath := envName(key), key
		if name == "" {
			return fmt.Errorf("key %q at %q has no letters or digits", key, path)
		}
		if prefix != "" {
			name, keyPath = prefix+"_"+name, path+"."+key
		}
		switch value.Kind {
		case yaml.MappingNode:
			if err := flattenMapping(value, name, keyPath, out, sources); err != nil {
				return err
			}
			continue
		case yaml.SequenceNode:
			items := make([]string, 0, len(value.Content))
			for _, item := range value.Content {
				item = resolve(item)
				if item.Kind != yaml.ScalarNode {
					return fmt.Errorf("%s: lists may only contain plain values", keyPath)
				}
				items = append(items, item.Value)
			}
			if err := set(out, sources, name, keyPath, strings.Join(items, ",")); err != nil {
				return err
			}
		case yaml.ScalarNode:
			if value.Tag == "!!null" {
				continue // not set: a lower-priority layer decides
			}
			if err := set(out, sources, name, keyPath, value.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func set(out, sources map[string]string, name, keyPath, value string) error {
	if other, ok := sources[name]; ok {
		return fmt.Errorf("%s and %s both map to %s", other, keyPath, name)
	}
	sources[name], out[name] = keyPath, value
	return nil
}

func resolve(node *yaml.Node) *yaml.Node {
	for node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

// envName converts one YAML key to its environment-variable form: every
// upper-case letter starts a new word, "_" and "-" separate words, and the
// words are joined with "_" in upper case. queryTimeout → QUERY_TIMEOUT,
// query_timeout → QUERY_TIMEOUT, mongoURI → MONGO_U_R_I.
func envName(key string) string {
	var words []string
	var word []rune
	flush := func() {
		if len(word) > 0 {
			words = append(words, strings.ToUpper(string(word)))
			word = word[:0]
		}
	}
	for _, r := range key {
		switch {
		case r == '_' || r == '-' || unicode.IsSpace(r):
			flush()
		case unicode.IsUpper(r):
			flush()
			word = append(word, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			word = append(word, r)
		}
	}
	flush()
	return strings.Join(words, "_")
}
