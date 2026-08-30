package repository

import (
	"context"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// DefinitionSnapshot is the durable Scheduler-owned configuration projection.
// Source identity is retained so manifest, remote and administrative publishers
// can be reconciled without making the Runtime host a persistence owner.
type DefinitionSnapshot struct {
	Revision      int64
	SchemaVersion string
	SchemaHash    string
	SourceKind    string
	SourceID      string
	Definitions   []schedulersdk.Definition
}

type DefinitionRepository interface {
	SyncDefinitions(context.Context, DefinitionSnapshot) error
	DefinitionSnapshot(context.Context) (DefinitionSnapshot, error)
}

// Binding is optional on the deployment-neutral Scheduler Binding and exposes
// source-owned persistence without expanding the execution protocol itself.
type Binding interface {
	DefinitionRepository() DefinitionRepository
}
