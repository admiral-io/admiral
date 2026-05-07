package model

import (
	"testing"
)

// TestParseResolvedValuesAsVars locks the wire-format contract between the
// server (this function) and the runner agent (which pastes each value
// verbatim into admiral.auto.tfvars.json). Every value the runner receives
// must be a valid JSON literal of the type Terraform expects: strings carry
// their quotes, numbers/bools are bare, complex values are object/array
// literals. Add a row here when extending support for a new shape; the
// runner-side test (admiral-infra-agent .../agent_test.go) mirrors this
// table so both sides stay in lockstep.
func TestParseResolvedValuesAsVars(t *testing.T) {
	cases := []struct {
		name string
		// raw is the JSON object string stored in revisions.resolved_values.
		raw string
		// want maps each variable name to the JSON literal the runner
		// should paste into tfvars.json. Order-insensitive.
		want map[string]string
	}{
		{
			name: "empty",
			raw:  "",
			want: map[string]string{},
		},
		{
			name: "string",
			raw:  `{"region":"us-west-2"}`,
			want: map[string]string{"region": `"us-west-2"`},
		},
		{
			name: "number_int",
			raw:  `{"count":42}`,
			want: map[string]string{"count": `42`},
		},
		{
			name: "number_float",
			raw:  `{"ratio":0.25}`,
			want: map[string]string{"ratio": `0.25`},
		},
		{
			name: "boolean_true",
			raw:  `{"enabled":true}`,
			want: map[string]string{"enabled": `true`},
		},
		{
			name: "boolean_false",
			raw:  `{"enabled":false}`,
			want: map[string]string{"enabled": `false`},
		},
		{
			name: "null",
			raw:  `{"opt":null}`,
			want: map[string]string{"opt": `null`},
		},
		{
			name: "list_of_strings",
			raw:  `{"zones":["us-west-2a","us-west-2b"]}`,
			want: map[string]string{"zones": `["us-west-2a","us-west-2b"]`},
		},
		{
			name: "list_of_numbers",
			raw:  `{"ports":[80,443]}`,
			want: map[string]string{"ports": `[80,443]`},
		},
		{
			name: "map_of_strings",
			raw:  `{"tags":{"team":"platform","cost-center":"42"}}`,
			want: map[string]string{"tags": `{"cost-center":"42","team":"platform"}`},
		},
		{
			name: "nested_object",
			raw:  `{"config":{"vpc":{"cidr":"10.0.0.0/16","tags":{"env":"prod"}}}}`,
			want: map[string]string{"config": `{"vpc":{"cidr":"10.0.0.0/16","tags":{"env":"prod"}}}`},
		},
		{
			name: "mixed_types_in_one_object",
			raw:  `{"region":"us-west-2","count":3,"enabled":true,"tags":{"env":"prod"}}`,
			want: map[string]string{
				"region":  `"us-west-2"`,
				"count":   `3`,
				"enabled": `true`,
				"tags":    `{"env":"prod"}`,
			},
		},
		{
			name: "string_containing_json",
			// Stored as a string; should remain a string at the wire, not
			// be re-decoded into a map.
			raw:  `{"note":"{\"not\":\"a-map\"}"}`,
			want: map[string]string{"note": `"{\"not\":\"a-map\"}"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseResolvedValuesAsVars(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("want %d keys, got %d (%v)", len(tc.want), len(got), got)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("var %q: want %s, got %s", k, want, got[k])
				}
			}
		})
	}
}

func TestParseResolvedValuesAsVars_RejectsNonObject(t *testing.T) {
	cases := []string{
		`["a","b"]`,
		`"plain-string"`,
		`42`,
		`{`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseResolvedValuesAsVars(raw); err == nil {
				t.Fatalf("expected error for %q", raw)
			}
		})
	}
}

func TestIsRevisionSatisfiedFor_PlanWithoutOutputRef(t *testing.T) {
	cases := []struct {
		blockerStatus string
		want          bool
	}{
		{RevisionStatusAwaitingApproval, true},
		{RevisionStatusApplying, true},
		{RevisionStatusSucceeded, true},
		{RevisionStatusPlanning, false},
		{RevisionStatusFailed, false},
	}
	for _, tc := range cases {
		got := IsRevisionSatisfiedFor(JobTypePlan, tc.blockerStatus, false)
		if got != tc.want {
			t.Errorf("plan/%s/no-output-ref = %v, want %v", tc.blockerStatus, got, tc.want)
		}
	}
}

func TestIsRevisionSatisfiedFor_PlanWithOutputRef(t *testing.T) {
	// b1 stiffening: when downstream references upstream's outputs,
	// promotion waits for SUCCEEDED (apply done + outputs captured).
	cases := []struct {
		blockerStatus string
		want          bool
	}{
		{RevisionStatusAwaitingApproval, false},
		{RevisionStatusApplying, false},
		{RevisionStatusSucceeded, true},
	}
	for _, tc := range cases {
		got := IsRevisionSatisfiedFor(JobTypePlan, tc.blockerStatus, true)
		if got != tc.want {
			t.Errorf("plan/%s/with-output-ref = %v, want %v", tc.blockerStatus, got, tc.want)
		}
	}
}

func TestIsRevisionSatisfiedFor_ApplyAlwaysRequiresSuccess(t *testing.T) {
	for _, requiresOutputs := range []bool{false, true} {
		if IsRevisionSatisfiedFor(JobTypeApply, RevisionStatusAwaitingApproval, requiresOutputs) {
			t.Errorf("apply should not unblock at AWAITING_APPROVAL (requiresOutputs=%v)", requiresOutputs)
		}
		if !IsRevisionSatisfiedFor(JobTypeApply, RevisionStatusSucceeded, requiresOutputs) {
			t.Errorf("apply should unblock at SUCCEEDED (requiresOutputs=%v)", requiresOutputs)
		}
	}
}
