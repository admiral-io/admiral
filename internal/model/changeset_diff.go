package model

import (
	changesetv1 "go.admiral.io/sdk/proto/admiral/changeset/v1"
)

const (
	DiffChangeTypeAdded   = "ADDED"
	DiffChangeTypeChanged = "CHANGED"
	DiffChangeTypeRemoved = "REMOVED"
)

var diffChangeTypeToProto = map[string]changesetv1.DiffChangeType{
	DiffChangeTypeAdded:   changesetv1.DiffChangeType_DIFF_CHANGE_TYPE_ADDED,
	DiffChangeTypeChanged: changesetv1.DiffChangeType_DIFF_CHANGE_TYPE_CHANGED,
	DiffChangeTypeRemoved: changesetv1.DiffChangeType_DIFF_CHANGE_TYPE_REMOVED,
}

type ChangeSetDiff struct {
	Entries    []EntryDiff
	Variables  []VariableDiff
	Downstream []DownstreamImpact
}

type EntryDiff struct {
	ComponentName    string
	ChangeType       string
	Module           *ModuleVersionDiff
	Values           []ValueDiff
	DependsOnAdded   []string
	DependsOnRemoved []string
	DescriptionOld   *string
	DescriptionNew   *string
}

type ModuleVersionDiff struct {
	ModuleIdOld   *string
	ModuleIdNew   *string
	ModuleNameOld *string
	ModuleNameNew *string
	VersionOld    *string
	VersionNew    *string
}

type ValueDiff struct {
	Key        string
	ChangeType string
	Old        *string
	New        *string
	Sensitive  bool
}

type VariableDiff struct {
	Key        string
	ChangeType string
	Old        *string
	New        *string
	Sensitive  bool
}

type DownstreamImpact struct {
	ComponentName string
	AffectedBy    []string
}

func (d *ChangeSetDiff) ToProto() *changesetv1.ChangeSetDiff {
	out := &changesetv1.ChangeSetDiff{}
	if len(d.Entries) > 0 {
		out.Entries = make([]*changesetv1.EntryDiff, 0, len(d.Entries))
		for i := range d.Entries {
			out.Entries = append(out.Entries, d.Entries[i].ToProto())
		}
	}
	if len(d.Variables) > 0 {
		out.Variables = make([]*changesetv1.VariableDiff, 0, len(d.Variables))
		for i := range d.Variables {
			out.Variables = append(out.Variables, d.Variables[i].ToProto())
		}
	}
	if len(d.Downstream) > 0 {
		out.Downstream = make([]*changesetv1.DownstreamImpact, 0, len(d.Downstream))
		for i := range d.Downstream {
			out.Downstream = append(out.Downstream, d.Downstream[i].ToProto())
		}
	}
	return out
}

func (e *EntryDiff) ToProto() *changesetv1.EntryDiff {
	out := &changesetv1.EntryDiff{
		ComponentName:    e.ComponentName,
		ChangeType:       changeSetEntryTypeToProto[e.ChangeType],
		DependsOnAdded:   e.DependsOnAdded,
		DependsOnRemoved: e.DependsOnRemoved,
	}
	if e.Module != nil {
		out.Module = e.Module.ToProto()
	}
	if len(e.Values) > 0 {
		out.Values = make([]*changesetv1.ValueDiff, 0, len(e.Values))
		for i := range e.Values {
			out.Values = append(out.Values, e.Values[i].ToProto())
		}
	}
	if e.DescriptionOld != nil {
		v := *e.DescriptionOld
		out.DescriptionOld = &v
	}
	if e.DescriptionNew != nil {
		v := *e.DescriptionNew
		out.DescriptionNew = &v
	}
	return out
}

func (m *ModuleVersionDiff) ToProto() *changesetv1.ModuleVersionDiff {
	out := &changesetv1.ModuleVersionDiff{}
	if m.ModuleIdOld != nil {
		v := *m.ModuleIdOld
		out.ModuleIdOld = &v
	}
	if m.ModuleIdNew != nil {
		v := *m.ModuleIdNew
		out.ModuleIdNew = &v
	}
	if m.ModuleNameOld != nil {
		v := *m.ModuleNameOld
		out.ModuleNameOld = &v
	}
	if m.ModuleNameNew != nil {
		v := *m.ModuleNameNew
		out.ModuleNameNew = &v
	}
	if m.VersionOld != nil {
		v := *m.VersionOld
		out.VersionOld = &v
	}
	if m.VersionNew != nil {
		v := *m.VersionNew
		out.VersionNew = &v
	}
	return out
}

func (v *ValueDiff) ToProto() *changesetv1.ValueDiff {
	out := &changesetv1.ValueDiff{
		Key:        v.Key,
		ChangeType: diffChangeTypeToProto[v.ChangeType],
		Sensitive:  v.Sensitive,
	}
	if v.Old != nil {
		s := *v.Old
		out.Old = &s
	}
	if v.New != nil {
		s := *v.New
		out.New = &s
	}
	return out
}

func (v *VariableDiff) ToProto() *changesetv1.VariableDiff {
	out := &changesetv1.VariableDiff{
		Key:        v.Key,
		ChangeType: diffChangeTypeToProto[v.ChangeType],
		Sensitive:  v.Sensitive,
	}
	if v.Old != nil {
		s := *v.Old
		out.Old = &s
	}
	if v.New != nil {
		s := *v.New
		out.New = &s
	}
	return out
}

func (d *DownstreamImpact) ToProto() *changesetv1.DownstreamImpact {
	return &changesetv1.DownstreamImpact{
		ComponentName: d.ComponentName,
		AffectedBy:    d.AffectedBy,
	}
}
