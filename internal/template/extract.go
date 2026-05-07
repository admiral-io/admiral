package template

import (
	"regexp"
	"sort"
)

type VarRef struct {
	Key string // e.g. "gcp_region"
}

type OutputRef struct {
	ComponentName string // referenced component, e.g. "vpc"
	Output        string // output name, e.g. "vpc_id"
}

var (
	varPattern       = regexp.MustCompile(`\.\bvar\.([a-zA-Z_][a-zA-Z0-9_]*)`)
	componentPattern = regexp.MustCompile(`\.\bcomponent\.([a-zA-Z_][a-zA-Z0-9_-]*)\.([a-zA-Z_][a-zA-Z0-9_]*)`)
)

func ExtractRefs(tmpl string) (vars []VarRef, outputs []OutputRef) {
	vars = extractVarRefs(tmpl)
	outputs = extractOutputRefs(tmpl)
	return vars, outputs
}

// ExtractReferencedComponents returns the unique component names whose outputs
// the given template reads via `{{ .component.<name>.<output> }}`. Used by the
// run engine to seed dependency edges and by changeset diff to compute
// downstream impact.
func ExtractReferencedComponents(tmpl string) []string {
	_, refs := ExtractRefs(tmpl)
	seen := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		seen[r.ComponentName] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for s := range seen {
		names = append(names, s)
	}
	sort.Strings(names)
	return names
}

func extractVarRefs(tmpl string) []VarRef {
	matches := varPattern.FindAllStringSubmatch(tmpl, -1)
	seen := make(map[string]struct{}, len(matches))
	var refs []VarRef
	for _, m := range matches {
		key := m[1]
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, VarRef{Key: key})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Key < refs[j].Key })
	return refs
}

func extractOutputRefs(tmpl string) []OutputRef {
	matches := componentPattern.FindAllStringSubmatch(tmpl, -1)
	type key struct{ s, o string }
	seen := make(map[key]struct{}, len(matches))
	var refs []OutputRef
	for _, m := range matches {
		k := key{m[1], m[2]}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		refs = append(refs, OutputRef{ComponentName: m[1], Output: m[2]})
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ComponentName != refs[j].ComponentName {
			return refs[i].ComponentName < refs[j].ComponentName
		}
		return refs[i].Output < refs[j].Output
	})
	return refs
}
