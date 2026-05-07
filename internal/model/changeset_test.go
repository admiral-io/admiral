package model

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestChangeSetEntryValidateDependsOn(t *testing.T) {
	csID := uuid.New()
	compID := uuid.New()
	modID := uuid.New()

	cases := []struct {
		name    string
		entry   ChangeSetEntry
		wantErr string
	}{
		{
			name: "create accepts slug deps",
			entry: ChangeSetEntry{
				ChangeSetId:   csID,
				ComponentName: "database",
				ChangeType:    ChangeSetEntryTypeCreate,
				ModuleId:      &modID,
				DependsOn:     pq.StringArray{"network", "security-group"},
			},
		},
		{
			name: "create rejects non-slug deps",
			entry: ChangeSetEntry{
				ChangeSetId:   csID,
				ComponentName: "database",
				ChangeType:    ChangeSetEntryTypeCreate,
				ModuleId:      &modID,
				DependsOn:     pq.StringArray{"123-leading-digit"},
			},
			wantErr: "invalid depends_on entry",
		},
		{
			name: "update accepts slug deps",
			entry: ChangeSetEntry{
				ChangeSetId:   csID,
				ComponentId:   &compID,
				ComponentName: "database",
				ChangeType:    ChangeSetEntryTypeUpdate,
				DependsOn:     pq.StringArray{"network"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}
