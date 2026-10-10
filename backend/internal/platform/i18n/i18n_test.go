package i18n

import (
	"reflect"
	"sort"
	"testing"
)

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Every language has exactly the keys of the default one and the same {placeholders}: a missing or renamed one is a
// build failure, not a blank string in production.
func TestCatalogsAreInParity(t *testing.T) {
	base := catalogs[Default]
	for _, tag := range Supported {
		c := catalogs[tag]
		if !reflect.DeepEqual(keys(c), keys(base)) {
			t.Errorf("%s: keys differ from %s", tag, Default)
		}
		for k, v := range c {
			a, b := placeholder.FindAllString(v, -1), placeholder.FindAllString(base[k], -1)
			sort.Strings(a)
			sort.Strings(b)
			if !reflect.DeepEqual(a, b) {
				t.Errorf("%s: %s has placeholders %v, %s has %v", tag, k, a, Default, b)
			}
			if v == "" {
				t.Errorf("%s: %s is empty", tag, k)
			}
		}
	}
}

func TestNegotiate(t *testing.T) {
	for in, want := range map[string]string{
		"": "en", "ru": "ru", "ru-RU,ru;q=0.9,en;q=0.8": "ru", "fr-FR,fr;q=0.9": "en", "en;q=0.4, ru;q=0.9": "ru", "de, ru;q=0.5": "ru", "ru;q=0": "en", "xx": "en",
	} {
		if got := Negotiate(in); got != want {
			t.Errorf("Negotiate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTFallsBackToDefaultThenKey(t *testing.T) {
	if got := T("ru", "problem.forbidden"); got != "Нет доступа" {
		t.Errorf("ru = %q", got)
	}
	if got := T("fr", "problem.forbidden"); got != "You do not have access to this" {
		t.Errorf("unknown language must fall back to English, got %q", got)
	}
	if got := T("ru", "no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key must come back as itself, got %q", got)
	}
	if got := T("en", "problem.forbidden", "x", "y"); got != "You do not have access to this" {
		t.Errorf("unused args must be ignored, got %q", got)
	}
}
