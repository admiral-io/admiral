package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// AppMeta exposes application metadata to template expressions via {{ .app.* }}.
type AppMeta struct {
	Name string
	Id   string
}

// EnvMeta exposes environment metadata to template expressions via {{ .env.* }}.
type EnvMeta struct {
	Name string
	Id   string
}

// RunMeta exposes run metadata to template expressions via {{ .run.* }}.
type RunMeta struct {
	Id string
}

// SelfMeta exposes the current component's own metadata via {{ .self.* }}.
type SelfMeta struct {
	Name string
	Slug string
}

// EvalContext holds every namespace reachable from a template expression.
//
//	{{ .var.KEY }}             → Var[KEY]
//	{{ .output.SLUG.OUT }}    → Output[SLUG][OUT]
//	{{ .app.name }}           → App.Name
//	{{ .env.name }}           → Env.Name
//	{{ .run.id }}             → Run.Id
//	{{ .self.name }}          → Self.Name
//	{{ .self.slug }}          → Self.Slug
type EvalContext struct {
	Var    map[string]any            // resolved variables (hierarchy-merged)
	Output map[string]map[string]any // component_slug → output_name → value
	App    AppMeta
	Env    EnvMeta
	Run    RunMeta
	Self   SelfMeta
}

// Evaluate executes tmpl as a Go text/template against ctx and returns the
// result. The output must be valid JSON; if it is not, Evaluate returns an
// error. Unresolved references are a hard error (the template is compiled with
// Option("missingkey=error")).
func Evaluate(tmpl string, ctx *EvalContext) (string, error) {
	if tmpl == "" {
		return "", nil
	}

	t, err := template.New("values").
		Option("missingkey=error").
		Funcs(FuncMap()).
		Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	// Build a map[string]any so template authors write {{ .var.x }},
	// {{ .output.vpc.id }}, {{ .app.name }} with lowercase namespace keys.
	varMap := ctx.Var
	if varMap == nil {
		varMap = map[string]any{}
	}
	outMap := ctx.Output
	if outMap == nil {
		outMap = map[string]map[string]any{}
	}

	data := map[string]any{
		"var":    varMap,
		"output": outMap,
		"app":    ctx.App,
		"env":    ctx.Env,
		"run":    ctx.Run,
		"self":   ctx.Self,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template evaluation error: %w", err)
	}

	out := buf.String()
	if !json.Valid([]byte(out)) {
		return "", fmt.Errorf("template output is not valid JSON: %s", truncate(out, 200))
	}
	return out, nil
}

func Validate(s string) error {
	if s == "" {
		return nil
	}
	_, err := template.New("validate").
		Funcs(FuncMap()).
		Parse(s)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
