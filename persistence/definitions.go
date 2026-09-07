package persistence

import (
	"context"

	schedulersdk "github.com/domainry/domainry-scheduler-sdk"
)

// DefinitionSnapshot is the durable Scheduler-owned configuration projection.
// Source identity is retained so manifest, remote and administrative publishers
// can be reconciled without making the Runtime host a persistence owner.
// PublisherFence contains only the session nonce hash; raw session nonces must
// never be written to Scheduler persistence or returned by snapshot reads.
type DefinitionSnapshot struct {
	PublisherFence *schedulersdk.DefinitionPublisherFence
	Revision       int64
	ContentSHA256  string
	SchemaVersion  string
	SchemaHash     string
	SourceKind     string
	SourceID       string
	Definitions    []schedulersdk.Definition
}

type DefinitionRepository interface {
	// SyncDefinitions must compare and persist PublisherFence, Revision and
	// ContentSHA256 in the same transaction as definition replacement whenever
	// the repository also implements DefinitionPublicationRepository.
	SyncDefinitions(context.Context, DefinitionSnapshot) error
	DefinitionSnapshot(context.Context) (DefinitionSnapshot, error)
}

// Binding is optional on the deployment-neutral Scheduler Binding and exposes
// source-owned persistence without expanding the execution protocol itself.
type Binding interface {
	DefinitionRepository() DefinitionRepository
}

// DefinitionPublicationRepository is an optional fenced-publication extension.
// Implementations are already bound to one authenticated Runtime application.
// BeginDefinitionPublisherSession must atomically allocate and persist a fence
// greater than every session previously issued for that application. Issuing a
// new session retires the old one; ActiveDefinitionPublisherSession survives a
// Scheduler restart and must never derive authority from a URL or header.
type DefinitionPublicationRepository interface {
	BeginDefinitionPublisherSession(context.Context) (schedulersdk.DefinitionPublisherSession, error)
	ActiveDefinitionPublisherSession(context.Context) (schedulersdk.DefinitionPublisherFence, bool, error)
}
