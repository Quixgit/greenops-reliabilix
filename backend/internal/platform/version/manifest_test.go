package version

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

// The version manifest is only worth having if it cannot drift: this test ties VERSIONS.md to the files and
// directories that really exist (docs/release/versioning.md, section 6).

var semver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-(alpha|beta|rc)\.[1-9]\d*)?$`)

const root = "../../../../"

func manifest(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(root + "VERSIONS.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	re := regexp.MustCompile("^\\| `([a-z0-9/_-]+)` \\| ([^|]+?) \\|")
	for _, line := range strings.Split(string(raw), "\n") {
		if m := re.FindStringSubmatch(line); m != nil {
			if _, dup := rows[m[1]]; dup {
				t.Errorf("VERSIONS.md lists %q twice", m[1])
			}
			rows[m[1]] = strings.TrimSpace(m[2])
		}
	}
	if len(rows) < 30 {
		t.Fatalf("only %d rows parsed from VERSIONS.md: format changed?", len(rows))
	}
	return rows
}

func TestPlatformVersionIsTheSameEverywhere(t *testing.T) {
	rows := manifest(t)
	file, err := os.ReadFile(root + "VERSION")
	if err != nil {
		t.Fatal(err)
	}
	v := strings.TrimSpace(string(file))
	if !semver.MatchString(v) {
		t.Fatalf("VERSION %q is not SemVer with an alpha/beta/rc stage", v)
	}
	if rows["platform"] != v {
		t.Errorf("VERSIONS.md platform = %q, VERSION = %q", rows["platform"], v)
	}
	pkg, err := os.ReadFile(root + "frontend/package.json")
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`"version":\s*"([^"]+)"`).FindStringSubmatch(string(pkg)); m == nil || m[1] != v {
		t.Errorf("frontend/package.json version = %v, VERSION = %q", m, v)
	}
	changelog, _ := os.ReadFile(root + "CHANGELOG.md")
	if !strings.Contains(string(changelog), "["+v+"]") && !strings.Contains(string(changelog), "## [Unreleased]") {
		t.Errorf("CHANGELOG.md has no entry for %s", v)
	}
}

func TestEveryRowIsValidAndAlphaRowsFollowThePlatform(t *testing.T) {
	rows := manifest(t)
	platform := rows["platform"]
	independent := regexp.MustCompile(`^(contract/|data/|base/)`)
	for key, v := range rows {
		switch {
		case independent.MatchString(key):
			// contracts, data and third-party images have their own numbering
		case !semver.MatchString(v):
			t.Errorf("%s: %q is not SemVer", key, v)
		case Stage(platform) == "alpha" && v != platform:
			t.Errorf("%s: %q, but the platform is in alpha (%s) and every row moves with it (policy section 2)", key, v, platform)
		}
	}
}

func dirs(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestEveryServiceModulePackageAndConnectorHasARow(t *testing.T) {
	rows := manifest(t)
	for _, d := range dirs(t, "../../../cmd") {
		if _, ok := rows["service/"+d]; !ok {
			t.Errorf("backend/cmd/%s has no `service/%s` row in VERSIONS.md", d, d)
		}
	}
	for _, d := range dirs(t, "../../") { // internal/
		if d == "platform" {
			continue
		}
		if _, ok := rows["internal/"+d]; !ok {
			t.Errorf("backend/internal/%s has no `internal/%s` row in VERSIONS.md", d, d)
		}
	}
	for _, d := range dirs(t, "../") { // internal/platform/
		if _, ok := rows["platform/"+d]; !ok {
			t.Errorf("backend/internal/platform/%s has no `platform/%s` row in VERSIONS.md", d, d)
		}
	}
	for key := range rows {
		switch {
		case strings.HasPrefix(key, "internal/"):
			if _, err := os.Stat("../../" + strings.TrimPrefix(key, "internal/")); err != nil {
				t.Errorf("%s is listed but the directory does not exist", key)
			}
		case strings.HasPrefix(key, "platform/"):
			if _, err := os.Stat("../" + strings.TrimPrefix(key, "platform/")); err != nil {
				t.Errorf("%s is listed but the directory does not exist", key)
			}
		case strings.HasPrefix(key, "service/") && key != "service/web" && key != "service/migrate":
			if _, err := os.Stat("../../../cmd/" + strings.TrimPrefix(key, "service/")); err != nil {
				t.Errorf("%s is listed but backend/cmd/%s does not exist", key, strings.TrimPrefix(key, "service/"))
			}
		}
	}
	// connectors and channels: their packages must exist where the manifest says
	for key, dir := range map[string]string{
		"connector/aws": "../../cloudaccounts/providers/aws", "connector/gcp": "../../cloudaccounts/providers/gcp",
		"connector/electricitymaps": "../../carbon/providers/electricitymaps",
		"channel/email":             "../../notifications/channels/email", "channel/webhook": "../../notifications/channels/webhook",
		"channel/slack": "../../notifications/channels/slack",
	} {
		if _, ok := rows[key]; !ok {
			t.Errorf("no `%s` row in VERSIONS.md", key)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	for _, d := range dirs(t, "../../cloudaccounts/providers") {
		if _, ok := rows["connector/"+d]; !ok {
			t.Errorf("connector %s has no `connector/%s` row in VERSIONS.md", d, d)
		}
	}
	for _, d := range dirs(t, "../../notifications/channels") {
		if d == "safehttp" {
			continue // a helper, not a channel
		}
		if _, ok := rows["channel/"+d]; !ok {
			t.Errorf("channel %s has no `channel/%s` row in VERSIONS.md", d, d)
		}
	}
}

func TestContractAndDataRowsMatchTheCode(t *testing.T) {
	rows := manifest(t)
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	if rows["contract/api"] != spec.Info.Version {
		t.Errorf("VERSIONS.md contract/api = %q, openapi.yaml info.version = %q", rows["contract/api"], spec.Info.Version)
	}
	if n, err := strconv.Atoi(rows["data/db-schema"]); err != nil || n != database.ExpectedMigration {
		t.Errorf("VERSIONS.md data/db-schema = %q, newest migration (database.ExpectedMigration) = %d", rows["data/db-schema"], database.ExpectedMigration)
	}
	files, _ := filepath.Glob("../../../migrations/*.sql")
	if len(files) != database.ExpectedMigration {
		t.Errorf("%d migration files, ExpectedMigration = %d", len(files), database.ExpectedMigration)
	}
	if !regexp.MustCompile(`^RLX-[A-Z]+-\d+$`).MatchString(rows["data/methodology"]) {
		t.Errorf("data/methodology = %q", rows["data/methodology"])
	}
}
