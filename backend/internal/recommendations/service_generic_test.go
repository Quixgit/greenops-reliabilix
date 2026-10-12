package recommendations

import (
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/azureprice"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
)

// The price job names a generic family by azureprice.Slug and the engine looks it up by domain.GenericFamily: the two must agree for every
// service the job builds, or the index is built and never found.
func TestGenericFamilyIdsAgree(t *testing.T) {
	if azureprice.GenericPrefix != domain.GenericPrefix {
		t.Fatalf("prefixes differ: %q %q", azureprice.GenericPrefix, domain.GenericPrefix)
	}
	for _, s := range azureprice.GenericServices {
		if got, want := domain.GenericFamily(s), azureprice.GenericPrefix+azureprice.Slug(s); got != want {
			t.Errorf("%q: engine %q, price job %q", s, got, want)
		}
	}
	for _, s := range azureprice.GenericServices {
		if domain.ManagedFamily("azure", s) != domain.GenericFamily(s) {
			t.Errorf("%q is built by the price job but the engine does not use it", s)
		}
	}
	if domain.ManagedFamily("azure", "Virtual Network") != "" || domain.ManagedFamily("azure", "Azure DevOps") != "" {
		t.Error("networking and DevOps must stay without an index")
	}
	for _, id := range azureprice.GenericIDs() {
		found := false
		for _, s := range azureprice.GenericServices {
			if domain.GenericFamily(s) == id {
				found = true
			}
		}
		if !found {
			t.Errorf("%q has no service", id)
		}
	}
}
