package template

import (
	"strings"
	"testing"
)

func TestEvaluate_EmptyTemplate(t *testing.T) {
	out, err := Evaluate("", &EvalContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty string, got %q", out)
	}
}

func TestEvaluate_PlainJSON(t *testing.T) {
	// Plain JSON with no expressions is a valid template that produces itself.
	tmpl := `{"region": "us-central1", "count": 3}`
	out, err := Evaluate(tmpl, &EvalContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != tmpl {
		t.Fatalf("expected %q, got %q", tmpl, out)
	}
}

func TestEvaluate_VarSubstitution(t *testing.T) {
	tmpl := `{"region": "{{ .var.gcp_region }}", "tier": "{{ .var.db_tier }}"}`
	ctx := &EvalContext{
		Var: map[string]any{
			"gcp_region": "europe-west1",
			"db_tier":    "db-custom-4-16384",
		},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"region": "europe-west1", "tier": "db-custom-4-16384"}`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEvaluate_EnvMeta(t *testing.T) {
	tmpl := `{"env": "{{ .env.Name }}"}`
	ctx := &EvalContext{
		Env: EnvMeta{Name: "staging", Id: "abc-123"},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"env": "staging"}`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEvaluate_ComponentRef(t *testing.T) {
	tmpl := `{"vpc_id": "{{ .component.vpc.vpc_id }}"}`
	ctx := &EvalContext{
		Component: map[string]map[string]any{
			"vpc": {"vpc_id": "vpc-abc123"},
		},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"vpc_id": "vpc-abc123"}`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEvaluate_ComplexTypeWithToJson(t *testing.T) {
	tmpl := `{"subnet_ids": {{ .component.vpc.subnet_ids | toJson }}}`
	ctx := &EvalContext{
		Component: map[string]map[string]any{
			"vpc": {
				"subnet_ids": []any{"subnet-a", "subnet-b", "subnet-c"},
			},
		},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"subnet_ids": ["subnet-a","subnet-b","subnet-c"]}`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEvaluate_Conditional(t *testing.T) {
	tmpl := `{"ha": {{ if eq .env.Name "production" }}true{{ else }}false{{ end }}}`
	ctx := &EvalContext{
		Env: EnvMeta{Name: "production"},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"ha": true}` {
		t.Fatalf("expected production=true, got %q", out)
	}

	ctx.Env.Name = "staging"
	out, err = Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"ha": false}` {
		t.Fatalf("expected staging=false, got %q", out)
	}
}

func TestEvaluate_StrictMissingVar(t *testing.T) {
	tmpl := `{"region": "{{ .var.nonexistent }}"}`
	ctx := &EvalContext{
		Var: map[string]any{"other": "value"},
	}
	_, err := Evaluate(tmpl, ctx)
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
	if !strings.Contains(err.Error(), "template evaluation error") {
		t.Fatalf("expected template evaluation error, got: %v", err)
	}
}

func TestEvaluate_StrictMissingComponent(t *testing.T) {
	tmpl := `{"vpc_id": "{{ .component.vpc.vpc_id }}"}`
	ctx := &EvalContext{}
	_, err := Evaluate(tmpl, ctx)
	if err == nil {
		t.Fatal("expected error for missing output, got nil")
	}
}

func TestEvaluate_InvalidTemplateOutput(t *testing.T) {
	// Template that produces invalid JSON.
	tmpl := `{{ .var.x }}`
	ctx := &EvalContext{
		Var: map[string]any{"x": "not json"},
	}
	_, err := Evaluate(tmpl, ctx)
	if err == nil {
		t.Fatal("expected error for non-JSON output, got nil")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("expected JSON validation error, got: %v", err)
	}
}

func TestEvaluate_InvalidTemplateSyntax(t *testing.T) {
	tmpl := `{{ .var.x `
	_, err := Evaluate(tmpl, &EvalContext{})
	if err == nil {
		t.Fatal("expected parse error for invalid syntax, got nil")
	}
	if !strings.Contains(err.Error(), "template parse error") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestEvaluate_PipelineChaining(t *testing.T) {
	tmpl := `{"host": "{{ .var.db_url | urlHost }}", "port": "{{ .var.db_url | urlPort }}"}`
	ctx := &EvalContext{
		Var: map[string]any{
			"db_url": "postgres://db.example.com:5432/mydb",
		},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"host": "db.example.com", "port": "5432"}`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEvaluate_DefaultHelper(t *testing.T) {
	tmpl := `{"region": "{{ .var.region | default "us-east-1" }}"}`
	// var.region is not set, so default kicks in -- but missingkey=error
	// means accessing .var.region will fail before default runs.
	// Test the case where the key exists but is empty.
	ctx := &EvalContext{
		Var: map[string]any{"region": ""},
	}
	out, err := Evaluate(tmpl, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"region": "us-east-1"}`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEvaluate_RequiredHelper(t *testing.T) {
	tmpl := `{"region": "{{ .var.region | required "region is required" }}"}`
	ctx := &EvalContext{
		Var: map[string]any{"region": ""},
	}
	_, err := Evaluate(tmpl, ctx)
	if err == nil {
		t.Fatal("expected error from required helper, got nil")
	}
	if !strings.Contains(err.Error(), "region is required") {
		t.Fatalf("expected required message, got: %v", err)
	}
}

func TestValidate_ValidTemplate(t *testing.T) {
	tests := []string{
		"",
		`{"static": "value"}`,
		`{"region": "{{ .var.x }}"}`,
		`{"ids": {{ .component.vpc.ids | toJson }}}`,
		`{{ if eq .env.Name "prod" }}{"ha": true}{{ else }}{"ha": false}{{ end }}`,
	}
	for _, tmpl := range tests {
		if err := Validate(tmpl); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", tmpl, err)
		}
	}
}

func TestValidate_InvalidTemplate(t *testing.T) {
	tests := []string{
		`{{ .var.x `,
		`{{ range }}`,
		`{{ end }}`,
	}
	for _, tmpl := range tests {
		if err := Validate(tmpl); err == nil {
			t.Errorf("Validate(%q) = nil, want error", tmpl)
		}
	}
}
