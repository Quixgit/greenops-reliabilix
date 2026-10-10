package version

import "testing"

func TestStage(t *testing.T) {
	for in, want := range map[string]string{
		"": "dev", "dev": "dev", "0.1.0-alpha.5": "alpha", "0.1.0-beta.3": "beta", "1.0.0-rc.2": "rc", "1.0.0": "stable", "1.2.3-weird": "dev", "0.1.0-alpha": "alpha",
	} {
		if got := Stage(in); got != want {
			t.Errorf("Stage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCurrentShortensTheCommit(t *testing.T) {
	old := Commit
	defer func() { Commit = old }()
	Commit = "0123456789abcdef0123"
	if got := Current().Commit; got != "0123456789ab" {
		t.Errorf("commit = %q", got)
	}
}
