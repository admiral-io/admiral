package template

import (
	"reflect"
	"testing"
)

func TestExtractRefs_VarOnly(t *testing.T) {
	tmpl := `{"region": "{{ .var.gcp_region }}", "tier": "{{ .var.db_tier }}"}`
	vars, outputs := ExtractRefs(tmpl)

	wantVars := []VarRef{{Key: "db_tier"}, {Key: "gcp_region"}}
	if !reflect.DeepEqual(vars, wantVars) {
		t.Errorf("vars = %v, want %v", vars, wantVars)
	}
	if len(outputs) != 0 {
		t.Errorf("expected no output refs, got %v", outputs)
	}
}

func TestExtractRefs_OutputOnly(t *testing.T) {
	tmpl := `{"vpc_id": "{{ .output.vpc.vpc_id }}", "subnets": {{ .output.vpc.subnet_ids | toJson }}}`
	vars, outputs := ExtractRefs(tmpl)

	if len(vars) != 0 {
		t.Errorf("expected no var refs, got %v", vars)
	}
	wantOutputs := []OutputRef{
		{Slug: "vpc", Output: "subnet_ids"},
		{Slug: "vpc", Output: "vpc_id"},
	}
	if !reflect.DeepEqual(outputs, wantOutputs) {
		t.Errorf("outputs = %v, want %v", outputs, wantOutputs)
	}
}

func TestExtractRefs_Mixed(t *testing.T) {
	tmpl := `{
		"vpc_id": "{{ .output.vpc.vpc_id }}",
		"region": "{{ .var.region }}",
		"db_host": "{{ .output.database.endpoint }}",
		"env": "{{ .var.environment }}"
	}`
	vars, outputs := ExtractRefs(tmpl)

	wantVars := []VarRef{{Key: "environment"}, {Key: "region"}}
	if !reflect.DeepEqual(vars, wantVars) {
		t.Errorf("vars = %v, want %v", vars, wantVars)
	}

	wantOutputs := []OutputRef{
		{Slug: "database", Output: "endpoint"},
		{Slug: "vpc", Output: "vpc_id"},
	}
	if !reflect.DeepEqual(outputs, wantOutputs) {
		t.Errorf("outputs = %v, want %v", outputs, wantOutputs)
	}
}

func TestExtractRefs_Deduplicated(t *testing.T) {
	tmpl := `{"a": "{{ .var.x }}", "b": "{{ .var.x }}", "c": "{{ .output.vpc.id }}", "d": "{{ .output.vpc.id }}"}`
	vars, outputs := ExtractRefs(tmpl)

	if len(vars) != 1 || vars[0].Key != "x" {
		t.Errorf("expected 1 var ref, got %v", vars)
	}
	if len(outputs) != 1 || outputs[0].Slug != "vpc" {
		t.Errorf("expected 1 output ref, got %v", outputs)
	}
}

func TestExtractRefs_Empty(t *testing.T) {
	vars, outputs := ExtractRefs(`{"static": "value"}`)
	if len(vars) != 0 {
		t.Errorf("expected no vars, got %v", vars)
	}
	if len(outputs) != 0 {
		t.Errorf("expected no outputs, got %v", outputs)
	}
}

func TestExtractRefs_WithPipeline(t *testing.T) {
	tmpl := `{"ids": {{ .output.vpc.subnet_ids | toJson }}, "region": "{{ .var.region | upper }}"}`
	vars, outputs := ExtractRefs(tmpl)

	if len(vars) != 1 || vars[0].Key != "region" {
		t.Errorf("expected 1 var ref (region), got %v", vars)
	}
	if len(outputs) != 1 || outputs[0].Slug != "vpc" || outputs[0].Output != "subnet_ids" {
		t.Errorf("expected 1 output ref (vpc.subnet_ids), got %v", outputs)
	}
}

func TestExtractOutputSlugs(t *testing.T) {
	tmpl := `{
		"vpc_id": "{{ .output.vpc.vpc_id }}",
		"db": "{{ .output.database.endpoint }}",
		"cidr": "{{ .output.vpc.cidr_block }}"
	}`
	slugs := ExtractOutputSlugs(tmpl)
	want := []string{"database", "vpc"}
	if !reflect.DeepEqual(slugs, want) {
		t.Errorf("slugs = %v, want %v", slugs, want)
	}
}

func TestExtractOutputSlugs_Empty(t *testing.T) {
	slugs := ExtractOutputSlugs(`{"static": "value"}`)
	if len(slugs) != 0 {
		t.Errorf("expected no slugs, got %v", slugs)
	}
}

func TestExtractRefs_HyphenatedSlug(t *testing.T) {
	tmpl := `{"id": "{{ .output.my-vpc.vpc_id }}"}`
	_, outputs := ExtractRefs(tmpl)
	if len(outputs) != 1 || outputs[0].Slug != "my-vpc" {
		t.Errorf("expected slug 'my-vpc', got %v", outputs)
	}
}
