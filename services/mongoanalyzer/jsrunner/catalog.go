package jsrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/entity"
)

var scriptName = regexp.MustCompile(`^(common|collections/[a-zA-Z][a-zA-Z0-9_.-]*)/[a-zA-Z][a-zA-Z0-9_-]*$`)
var collectionSegment = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
var fileStem = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// Top-level script directories, matching the scriptName pattern above.
const (
	dirCommon      = "common"
	dirCollections = "collections"
)

// Catalog resolves analysis script names against a scripts file tree:
// discovery, existence, collection-target validation, and plans.
type Catalog struct {
	scripts fs.FS
}

func NewCatalog(scripts fs.FS) *Catalog { return &Catalog{scripts: scripts} }

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
		if scriptName.MatchString(name) {
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
	if !scriptName.MatchString(name) {
		return false
	}
	file, err := fs.Stat(c.scripts, name+".js")
	return err == nil && file.Mode().IsRegular()
}

func (c *Catalog) ValidateTarget(name, collection string) error {
	if !scriptName.MatchString(name) {
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

func (c *Catalog) readPlan(file, group string) ([]string, error) {
	data, err := fs.ReadFile(c.scripts, file)
	if err != nil {
		return nil, err
	}
	var plan map[string][]string
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	names := plan[group]
	if len(names) == 0 {
		return nil, fmt.Errorf("%s has no scripts for %q", file, group)
	}
	return names, nil
}

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
	if !collectionSegment.MatchString(collection) || strings.Contains(collection, "..") {
		return nil, fmt.Errorf("collection %q cannot be used as a script directory name", collection)
	}
	file := path.Join(dirCollections, collection, "plan.json")
	workloads, err := c.readPlan(file, entity.PlanGroupDefault)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no analysis plan for collection %q; use make inspect or add %s", collection, file)
	}
	if err != nil {
		return nil, err
	}
	for i, name := range workloads {
		if !fileStem.MatchString(name) {
			return nil, fmt.Errorf("invalid workload script %q in %s", name, file)
		}
		workloads[i] = dirCollections + "/" + collection + "/" + name
		if !c.Exists(workloads[i]) {
			return nil, fmt.Errorf("script %q not found", workloads[i])
		}
	}
	return append(common, workloads...), nil
}
