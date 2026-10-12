package application

import "testing"

func TestInviteLink(t *testing.T) {
	for base, want := range map[string]string{
		"https://app.example.com":  "https://app.example.com/invite?token=a%2Bb%26c",
		"https://app.example.com/": "https://app.example.com/invite?token=a%2Bb%26c",
	} {
		if got := InviteLink(base, "a+b&c"); got != want {
			t.Errorf("%q: %q", base, got)
		}
	}
}
