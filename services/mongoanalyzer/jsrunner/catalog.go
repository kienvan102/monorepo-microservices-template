package jsrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/entity"
)

// A script name is common/<script> or collections/<collection>/<script>. An
// item name, collections/<collection>/<group>/<item>, names one file of a
// group declared in that collection's plan.json.
var scriptName = regexp.MustCompile(`^(common|collections/[a-zA-Z][a-zA-Z0-9_.-]*)/[a-zA-Z][a-zA-Z0-9_-]*$`)
var itemName = regexp.MustCompile(`^collections/([a-zA-Z][a-zA-Z0-9_.-]*)/([a-zA-Z][a-zA-Z0-9_-]*)/([a-zA-Z][a-zA-Z0-9_-]*)$`)
var collectionSegment = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
var fileStem = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
var jsIdentifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// Top-level script directories, matching the scriptName pattern above.
const (
	dirCommon      = "common"
	dirCollections = "collections"
)

// planFile is the shape of a plan.json.
type planFile struct {
	Inspect []string             `json:"inspect"`
	Default []string             `json:"default"`
	Groups  map[string]groupSpec `json:"groups"`
}

// groupSpec is a group as written in a collection's plan.json.
type groupSpec struct {
	Engines []string `json:"engines"`
	Global  string   `json:"global"`
	Items   []string `json:"items"`
}

// Group is a folder of items under a collection, collections/<c>/<group>/,
// declared in the collection's plan.json. Engines are the scripts that run
// the items (the first one runs a single item named on its own), Global is
// the JS variable the items are handed in, and Items are the planned items
// in order. An item is a single JS expression; files in the folder that the
// plan does not list can still be run by name.
type Group struct {
	Name    string
	Engines []string // full script names, collections/<c>/<engine>
	Global  string
	Items   []string
}

// Item is one file of a group: its name and JS source.
type Item struct {
	Name, Source string
}

// Resolved is what runs for a script name: the JS source to run, and the
// group items to hand it in the JS variable Global. Both are empty for a
// script that is not a group's engine.
type Resolved struct {
	Source string
	Global string
	Items  []Item
}

// Catalog resolves analysis script names against a scripts file tree:
// discovery, existence, collection-target validation, plans, and groups. It
// knows what a group is, never which groups exist: those come from
// plan.json.
type Catalog struct {
	scripts fs.FS
}

func NewCatalog(scripts fs.FS) *Catalog { return &Catalog{scripts: scripts} }

// Scripts

func (c *Catalog) Discover() ([]string, error) {
	var result []string
	err := fs.WalkDir(c.scripts, ".", func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			return nil
		}
		name := strings.TrimSuffix(p, ".js")
		if c.Exists(name) {
			result = append(result, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func (c *Catalog) Exists(name string) bool {
	if itemName.MatchString(name) {
		_, _, err := c.findItem(name)
		return err == nil
	}
	return scriptName.MatchString(name) && c.regularFile(name+".js")
}

func (c *Catalog) ValidateTarget(name, collection string) error {
	if !scriptName.MatchString(name) && !itemName.MatchString(name) {
		return fmt.Errorf("invalid script name %q", name)
	}
	parts := strings.Split(name, "/")
	if parts[0] == dirCollections && parts[1] != collection {
		return fmt.Errorf("script %q is for %s; .env selects %s", name, parts[1], collection)
	}
	return nil
}

// ReadSource returns a script's JS source and its path inside the scripts
// tree (used as the VM's script name for readable stack traces).
func (c *Catalog) ReadSource(name string) (source, file string, err error) {
	file = name + ".js"
	data, err := fs.ReadFile(c.scripts, file)
	if err != nil {
		return "", file, err
	}
	return string(data), file, nil
}

// Plans

func (c *Catalog) Plan(group, collection string) ([]string, error) {
	common, err := c.readPlan("plan.json", entity.PlanGroupInspect)
	if err != nil {
		return nil, err
	}
	for _, name := range common {
		if !strings.HasPrefix(name, dirCommon+"/") || !c.Exists(name) {
			return nil, fmt.Errorf("invalid common script %q in scripts/plan.json", name)
		}
	}
	if group == entity.PlanGroupInspect {
		return common, nil
	}
	if group != entity.PlanGroupDefault {
		return nil, fmt.Errorf("unknown plan group %q", group)
	}
	if err := c.checkCollection(collection); err != nil {
		return nil, err
	}
	file := path.Join(dirCollections, collection, "plan.json")
	scripts, err := c.readPlan(file, entity.PlanGroupDefault)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no analysis plan for collection %q; use make inspect or add %s", collection, file)
	}
	if err != nil {
		return nil, err
	}
	for i, name := range scripts {
		if !fileStem.MatchString(name) {
			return nil, fmt.Errorf("invalid workload script %q in %s", name, file)
		}
		scripts[i] = dirCollections + "/" + collection + "/" + name
		if !c.Exists(scripts[i]) {
			return nil, fmt.Errorf("script %q not found", scripts[i])
		}
	}
	return append(common, scripts...), nil
}

// Groups

// Resolve works out what runs for a script name. An item name runs the
// default engine of its group on that one item. A script named as an engine
// by a group gets all of that group's planned items. Any other script runs
// as written, with nothing handed to it.
func (c *Catalog) Resolve(name string) (Resolved, error) {
	if m := itemName.FindStringSubmatch(name); m != nil {
		group, item, err := c.findItem(name)
		if err != nil {
			return Resolved{}, err
		}
		source, _, err := c.ReadSource(group.Engines[0])
		if err != nil {
			return Resolved{}, err
		}
		text, _, err := c.ReadSource(name)
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{Source: source, Global: group.Global, Items: []Item{{Name: item, Source: text}}}, nil
	}

	source, _, err := c.ReadSource(name)
	if err != nil {
		return Resolved{}, err
	}
	resolved := Resolved{Source: source}
	parts := strings.Split(name, "/")
	if parts[0] != dirCollections {
		return resolved, nil
	}
	groups, err := c.groups(parts[1])
	if err != nil {
		return Resolved{}, err
	}
	for _, group := range groups {
		if !slices.Contains(group.Engines, name) {
			continue
		}
		resolved.Global = group.Global
		for _, item := range group.Items {
			text, _, err := c.ReadSource(path.Join(dirCollections, parts[1], group.Name, item))
			if err != nil {
				return Resolved{}, err
			}
			resolved.Items = append(resolved.Items, Item{Name: item, Source: text})
		}
		break
	}
	return resolved, nil
}

// Helpers

// readPlan returns the scripts a plan.json lists under group, which must not
// be empty.
func (c *Catalog) readPlan(file, group string) ([]string, error) {
	plan, err := c.readPlanFile(file)
	if err != nil {
		return nil, err
	}
	var names []string
	switch group {
	case entity.PlanGroupInspect:
		names = plan.Inspect
	case entity.PlanGroupDefault:
		names = plan.Default
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s has no scripts for %q", file, group)
	}
	return names, nil
}

func (c *Catalog) readPlanFile(file string) (planFile, error) {
	var plan planFile
	data, err := fs.ReadFile(c.scripts, file)
	if err != nil {
		return plan, err
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, fmt.Errorf("%s: %w", file, err)
	}
	return plan, nil
}

// groups returns the groups declared in a collection's plan.json, by name. A
// collection without a plan.json, or without groups, has none. A declaration
// that cannot work is an error that names the file.
func (c *Catalog) groups(collection string) (map[string]Group, error) {
	if err := c.checkCollection(collection); err != nil {
		return nil, err
	}
	file := path.Join(dirCollections, collection, "plan.json")
	plan, err := c.readPlanFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(plan.Groups))
	for name := range plan.Groups {
		names = append(names, name)
	}
	sort.Strings(names)

	groups := make(map[string]Group, len(names))
	engineOf := map[string]string{}
	for _, name := range names {
		group, err := c.newGroup(collection, name, plan.Groups[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		for _, engine := range group.Engines {
			if other, taken := engineOf[engine]; taken {
				return nil, fmt.Errorf("%s: %s is an engine of both group %q and group %q", file, engine, other, name)
			}
			engineOf[engine] = name
		}
		groups[name] = group
	}
	return groups, nil
}

func (c *Catalog) newGroup(collection, name string, spec groupSpec) (Group, error) {
	if !fileStem.MatchString(name) {
		return Group{}, fmt.Errorf("invalid group name %q", name)
	}
	if !jsIdentifier.MatchString(spec.Global) || spec.Global == optionsGlobal {
		return Group{}, fmt.Errorf("group %q: %q cannot be used as the JS variable for its items", name, spec.Global)
	}
	if len(spec.Engines) == 0 {
		return Group{}, fmt.Errorf("group %q has no engines", name)
	}
	group := Group{Name: name, Global: spec.Global, Items: spec.Items}
	for _, engine := range spec.Engines {
		script := path.Join(dirCollections, collection, engine)
		if !fileStem.MatchString(engine) || !c.Exists(script) {
			return Group{}, fmt.Errorf("group %q: engine %q not found", name, engine)
		}
		group.Engines = append(group.Engines, script)
	}
	for _, item := range spec.Items {
		if !fileStem.MatchString(item) || !c.regularFile(path.Join(dirCollections, collection, name, item+".js")) {
			return Group{}, fmt.Errorf("group %q: item %q not found", name, item)
		}
	}
	return group, nil
}

// findItem returns the group an item name belongs to, and the item's own
// name, if the group is declared in the collection's plan.json and the item
// file exists.
func (c *Catalog) findItem(name string) (Group, string, error) {
	m := itemName.FindStringSubmatch(name)
	if m == nil {
		return Group{}, "", fmt.Errorf("%q is not an item name", name)
	}
	groups, err := c.groups(m[1])
	if err != nil {
		return Group{}, "", err
	}
	group, declared := groups[m[2]]
	if !declared {
		return Group{}, "", fmt.Errorf("%q is not a group declared in collections/%s/plan.json", m[2], m[1])
	}
	if !c.regularFile(name + ".js") {
		return Group{}, "", fmt.Errorf("item %q not found in group %q", m[3], m[2])
	}
	return group, m[3], nil
}

func (c *Catalog) checkCollection(collection string) error {
	if !collectionSegment.MatchString(collection) || strings.Contains(collection, "..") {
		return fmt.Errorf("collection %q cannot be used as a script directory name", collection)
	}
	return nil
}

func (c *Catalog) regularFile(file string) bool {
	info, err := fs.Stat(c.scripts, file)
	return err == nil && info.Mode().IsRegular()
}
