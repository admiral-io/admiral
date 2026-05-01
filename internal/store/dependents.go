package store

import (
	"fmt"
	"strings"
)

// DependentsError is returned by cascade-delete operations when one or more
// blockers prevent the delete from proceeding. The Children list describes
// what's blocking; entries may include guidance for the operator (e.g.
// "use force to delete; cloud resources will leak"). Per-blocker context
// belongs in the Children entry that produced it.
type DependentsError struct {
	Resource string
	Name     string
	Children []string
}

func (e *DependentsError) Error() string {
	return fmt.Sprintf("cannot delete %s %q: blocked by %s",
		e.Resource, e.Name, strings.Join(e.Children, ", "))
}

// DeleteResult reports the cascade counts produced by a successful
// cascade-delete. EnvironmentStore.Delete populates Runs only;
// ApplicationStore.Delete populates both fields. Zero-valued fields mean
// the dependent kind is not tracked by the operation that produced the
// result.
type DeleteResult struct {
	Environments int64
	Runs         int64
}
