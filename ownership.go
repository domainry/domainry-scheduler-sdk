package schedulersdk

import "strings"

var ownedManifestObjectKeys = []string{"job_definition", "scheduler_cursor", "job_run", "job_run_event", "job_dead_letter"}

var ownedManifestObjectKeySet = func() map[string]struct{} {
	result := make(map[string]struct{}, len(ownedManifestObjectKeys))
	for _, key := range ownedManifestObjectKeys {
		result[key] = struct{}{}
	}
	return result
}()

// OwnsManifestObjectKey reports whether a logical object name belongs to
// Scheduler definition/execution persistence and therefore must not be
// declared or seeded as a Runtime Record object.
func OwnsManifestObjectKey(key string) bool {
	_, owned := ownedManifestObjectKeySet[strings.TrimSpace(key)]
	return owned
}

func OwnedManifestObjectKeys() []string {
	return append([]string(nil), ownedManifestObjectKeys...)
}
