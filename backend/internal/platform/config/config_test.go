package config

import (
	"strings"
	"testing"
)

func TestValidateNotifications(t *testing.T) {
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 zero bytes, base64
	cases := []struct {
		name string
		c    Config
		bad  string // substring of the expected error; "" = valid
	}{
		{"nothing configured", Config{}, ""},
		{"valid key", Config{NotificationsKey: key}, ""},
		{"short key", Config{NotificationsKey: "c2hvcnQ="}, "NOTIFICATIONS_SECRET_KEY"},
		{"https base url", Config{PublicBaseURL: "https://app.example.com"}, ""},
		{"https base url with slash", Config{PublicBaseURL: "https://app.example.com/"}, ""},
		{"http base url in prod", Config{Env: "prod", PublicBaseURL: "http://app.example.com"}, "PUBLIC_BASE_URL"},
		{"http base url in dev", Config{Env: "dev", PublicBaseURL: "http://localhost:3000"}, ""},
		{"base url with path", Config{PublicBaseURL: "https://app.example.com/x"}, "PUBLIC_BASE_URL"},
		{"base url with credentials", Config{PublicBaseURL: "https://u:p@app.example.com"}, "PUBLIC_BASE_URL"},
		{"base url without host", Config{PublicBaseURL: "https://"}, "PUBLIC_BASE_URL"},
		{"base url garbage", Config{PublicBaseURL: "://nope"}, "PUBLIC_BASE_URL"},
		{"smtp complete", Config{SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPFrom: "Reliabilix <n@example.com>"}, ""},
		{"smtp without sender", Config{SMTPHost: "smtp.example.com", SMTPPort: 587}, "SMTP_FROM"},
		{"smtp bad sender", Config{SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPFrom: "nope"}, "SMTP_FROM"},
		{"smtp bad port", Config{SMTPHost: "smtp.example.com", SMTPPort: 70000, SMTPFrom: "n@example.com"}, "SMTP_PORT"},
	}
	for _, c := range cases {
		err := c.c.validateNotifications()
		switch {
		case c.bad == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.name, err)
		case c.bad != "" && (err == nil || !strings.Contains(err.Error(), c.bad)):
			t.Errorf("%s: err = %v, want it to mention %s", c.name, err, c.bad)
		}
	}
}
