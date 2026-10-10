package methodology

import "testing"

// A published coefficient set is immutable (ADR-0009): its bytes are pinned here. Changing a number means a NEW file
// with a new version name, never an edit; this test then needs a new line, not a changed one.
var pinned = map[string]string{
	"RLX-PROVISIONAL-1": "99269da432c463c1536593932a4a44911b9a92f4aaefc0e5178dab821006f526",
}

func TestPublishedSetsAreImmutable(t *testing.T) {
	for _, f := range Versions() {
		want, ok := pinned[f.Version]
		if !ok {
			t.Errorf("%s is not pinned: add its checksum %s", f.Version, Checksum(f.Version))
			continue
		}
		if got := Checksum(f.Version); got != want {
			t.Errorf("%s was edited (sha256 %s, pinned %s): publish a new version instead", f.Version, got, want)
		}
	}
	for v := range pinned {
		if _, ok := Get(v); !ok {
			t.Errorf("pinned version %s is gone: a published set is never removed", v)
		}
	}
}

func TestActiveVersionExists(t *testing.T) {
	if f, ok := Get(ActiveVersion); !ok || f.Version != Version {
		t.Fatalf("active version %q must resolve", ActiveVersion)
	}
}
