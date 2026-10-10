// Package version carries what a running process knows about its own build: the platform release, which service
// it is and which commit it was built from. The values are injected at build time (see deploy/docker/Dockerfile):
//
//	-X .../platform/version.Platform=0.1.0-alpha.15 -X .../platform/version.Service=api -X .../platform/version.Commit=<sha>
//
// Release rules are in docs/release/versioning.md; the manifest of every service, module and image version is
// VERSIONS.md.
package version

import "strings"

var (
	// Platform is the release the whole platform belongs to (SemVer, for example 0.1.0-alpha.15). "dev" for local builds.
	Platform = "dev"
	// Service is the process: api, worker, scheduler, admin ... "unknown" when not injected.
	Service = "unknown"
	// Commit is the git revision the binary was built from.
	Commit = "unknown"
)

// Info is the build identity as served by /version and printed at start-up.
type Info struct {
	Platform string `json:"platform"`
	Stage    string `json:"stage"` // dev | alpha | beta | rc | stable
	Service  string `json:"service"`
	Commit   string `json:"commit"`
}

// Stage derives the release stage from the SemVer pre-release identifier: 0.1.0-alpha.15 -> alpha.
func Stage(v string) string {
	if v == "" || v == "dev" {
		return "dev"
	}
	_, pre, ok := strings.Cut(v, "-")
	if !ok {
		return "stable"
	}
	switch id, _, _ := strings.Cut(pre, "."); id {
	case "alpha", "beta", "rc":
		return id
	}
	return "dev"
}

// Current returns this process's build identity.
func Current() Info {
	return Info{Platform: Platform, Stage: Stage(Platform), Service: Service, Commit: short(Commit)}
}

// Attrs is Current as slog key/value pairs.
func Attrs() []any {
	i := Current()
	return []any{"version", i.Platform, "stage", i.Stage, "service", i.Service, "commit", i.Commit}
}

func short(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}
