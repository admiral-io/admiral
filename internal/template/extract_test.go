package template

import (
	"reflect"
	"testing"
)

func TestExtractRefs_VarOnly(t *testing.T) {
	tmpl := `{"region": "{{ .var.gcp_region }}", "tier": "{{ .var.db_tier }}"}`
	vars, comps := ExtractRefs(tmpl)

	wantVars := []VarRef{{Key: "db_tier"}, {Key: "gcp_region"}}
	if !reflect.DeepEqual(vars, wantVars) {
		t.Errorf("vars = %v, want %v", vars, wantVars)
	}
	if len(comps) != 0 {
		t.Errorf("expected no component refs, got %v", comps)
	}
}

func TestExtractRefs_ComponentOnly(t *testing.T) {
	tmpl := `{"vpc_id": "{{ .component.vpc.vpc_id }}", "subnets": {{ .component.vpc.subnet_ids | toJson }}}`
	vars, comps := ExtractRefs(tmpl)

	if len(vars) != 0 {
		t.Errorf("expected no var refs, got %v", vars)
	}
	wantComps := []ComponentRef{
		{Component: "vpc", Output: "subnet_ids"},
		{Component: "vpc", Output: "vpc_id"},
	}
	if !reflect.DeepEqual(comps, wantComps) {
		t.Errorf("comps = %v, want %v", comps, wantComps)
	}
}

func TestExtractRefs_Mixed(t *testing.T) {
	tmpl := `{
		"vpc_id": "{{ .component.vpc.vpc_id }}",
		"region": "{{ .var.region }}",
		"db_host": "{{ .component.database.endpoint }}",
		"env": "{{ .var.environment }}"
	}`
	vars, comps := ExtractRefs(tmpl)

	wantVars := []VarRef{{Key: "environment"}, {Key: "region"}}
	if !reflect.DeepEqual(vars, wantVars) {
		t.Errorf("vars = %v, want %v", vars, wantVars)
	}

	wantComps := []ComponentRef{
		{Component: "database", Output: "endpoint"},
		{Component: "vpc", Output: "vpc_id"},
	}
	if !reflect.DeepEqual(comps, wantComps) {
		t.Errorf("comps = %v, want %v", comps, wantComps)
	}
}

func TestExtractRefs_Deduplicated(t *testing.T) {
	tmpl := `{"a": "{{ .var.x }}", "b": "{{ .var.x }}", "c": "{{ .component.vpc.id }}", "d": "{{ .component.vpc.id }}"}`
	vars, comps := ExtractRefs(tmpl)

	if len(vars) != 1 || vars[0].Key != "x" {
		t.Errorf("expected 1 var ref, got %v", vars)
	}
	if len(comps) != 1 || comps[0].Component != "vpc" {
		t.Errorf("expected 1 component ref, got %v", comps)
	}
}

func TestExtractRefs_Empty(t *testing.T) {
	vars, comps := ExtractRefs(`{"static": "value"}`)
	if len(vars) != 0 {
		t.Errorf("expected no vars, got %v", vars)
	}
	if len(comps) != 0 {
		t.Errorf("expected no comps, got %v", comps)
	}
}

func TestExtractRefs_WithPipeline(t *testing.T) {
	tmpl := `{"ids": {{ .component.vpc.subnet_ids | toJson }}, "region": "{{ .var.region | upper }}"}`
	vars, comps := ExtractRefs(tmpl)

	if len(vars) != 1 || vars[0].Key != "region" {
		t.Errorf("expected 1 var ref (region), got %v", vars)
	}
	if len(comps) != 1 || comps[0].Component != "vpc" || comps[0].Output != "subnet_ids" {
		t.Errorf("expected 1 component ref (vpc.subnet_ids), got %v", comps)
	}
}

func TestExtractComponentNames(t *testing.T) {
	tmpl := `{
		"vpc_id": "{{ .component.vpc.vpc_id }}",
		"db": "{{ .component.database.endpoint }}",
		"cidr": "{{ .component.vpc.cidr_block }}"
	}`
	names := ExtractComponentNames(tmpl)
	want := []string{"database", "vpc"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestExtractComponentNames_Empty(t *testing.T) {
	names := ExtractComponentNames(`{"static": "value"}`)
	if len(names) != 0 {
		t.Errorf("expected no names, got %v", names)
	}
}

func TestExtractRefs_HyphenatedComponentName(t *testing.T) {
	tmpl := `{"id": "{{ .component.my-vpc.vpc_id }}"}`
	_, comps := ExtractRefs(tmpl)
	if len(comps) != 1 || comps[0].Component != "my-vpc" {
		t.Errorf("expected component 'my-vpc', got %v", comps)
	}
}
