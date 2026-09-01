package schedulersdk

import "testing"

func TestOwnedManifestObjectKeysAreSingleSourceForHostGuards(t *testing.T) {
	keys := OwnedManifestObjectKeys()
	if len(keys) != 5 {
		t.Fatalf("keys=%v", keys)
	}
	for _, key := range keys {
		if !OwnsManifestObjectKey(" " + key + " ") {
			t.Errorf("key %q is not recognized", key)
		}
	}
	if OwnsManifestObjectKey("record_timer") {
		t.Fatal("Runtime Record timers must not be transferred to Scheduler ownership")
	}
}
