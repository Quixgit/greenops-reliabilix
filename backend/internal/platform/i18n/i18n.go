// Package i18n is the server side of the platform's multi-language support (ADR-0036). The server speaks in stable
// codes and English defaults; this package supplies translations for the few texts the server itself produces (problem
// titles, e-mail and report wording) and picks the language from a request.
//
// A language is added by adding locales/<tag>.json (every key of en.json) and its tag to Supported. A test fails when
// a catalog lacks a key or changes a placeholder.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Default is the language every text exists in and every missing translation falls back to.
const Default = "en"

// Supported lists the language tags with a catalog, in display order.
var Supported = []string{"en", "ru"}

//go:embed locales/*.json
var files embed.FS

var catalogs = load()

func load() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, tag := range Supported {
		b, err := files.ReadFile("locales/" + tag + ".json")
		if err != nil {
			panic(fmt.Sprintf("i18n: missing catalog for %q", tag))
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			panic(fmt.Sprintf("i18n: bad catalog %q: %v", tag, err))
		}
		out[tag] = m
	}
	return out
}

// Has reports whether a language tag is supported.
func Has(tag string) bool {
	_, ok := catalogs[tag]
	return ok
}

// Negotiate picks the best supported language for an Accept-Language header value; Default when nothing matches.
// Only the primary subtag is compared ("ru-RU" matches "ru"); quality values order the choices.
func Negotiate(header string) string {
	type cand struct {
		tag string
		q   float64
		pos int
	}
	var cs []cand
	for i, part := range strings.Split(header, ",") {
		tag, q := strings.TrimSpace(part), 1.0
		if semi := strings.Index(tag, ";"); semi >= 0 {
			if v, ok := strings.CutPrefix(strings.TrimSpace(tag[semi+1:]), "q="); ok {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					q = f
				}
			}
			tag = strings.TrimSpace(tag[:semi])
		}
		if dash := strings.IndexAny(tag, "-_"); dash >= 0 {
			tag = tag[:dash]
		}
		if tag = strings.ToLower(tag); Has(tag) && q > 0 {
			cs = append(cs, cand{tag, q, i})
		}
	}
	sort.SliceStable(cs, func(a, b int) bool { return cs[a].q > cs[b].q })
	if len(cs) == 0 {
		return Default
	}
	return cs[0].tag
}

var placeholder = regexp.MustCompile(`\{([a-z_][a-z0-9_]*)\}`)

// T returns the text for key in the language, falling back to Default, and finally to the key itself (a visible, safe
// failure). {name} placeholders are replaced from args given as name, value pairs.
func T(lang, key string, args ...any) string {
	s, ok := catalogs[lang][key]
	if !ok {
		if s, ok = catalogs[Default][key]; !ok {
			return key
		}
	}
	if len(args) < 2 {
		return s
	}
	vals := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		vals[fmt.Sprint(args[i])] = fmt.Sprint(args[i+1])
	}
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := vals[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

// Lookup is T that reports whether the key exists in the language itself (no fallback).
func Lookup(lang, key string) (string, bool) {
	s, ok := catalogs[lang][key]
	return s, ok
}
