package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/safehttp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

func TestPreparePinsTheSlackHost(t *testing.T) {
	c := New(safehttp.Options{})
	_, hint, err := c.Prepare("https://hooks.slack.com/services/T000/B000/XXXXXXXX")
	if err != nil || hint != "hooks.slack.com" {
		t.Errorf("valid URL: hint=%q err=%v", hint, err)
	}
	for _, bad := range []string{
		"https://evil.example.com/services/T/B/X", "https://hooks.slack.com.evil.com/services/T/B/X", "https://hooks.slack.com/other/T/B/X",
		"https://hooks.slack.com:8443/services/T/B/X", "https://hooks.slack.com/services/T/B/X?x=1", "http://hooks.slack.com/services/T/B/X", "",
	} {
		if _, _, err := c.Prepare(bad); err == nil {
			t.Errorf("Prepare(%q) must fail", bad)
		}
	}
}

func TestTextEscapesUserControlledContent(t *testing.T) {
	got := Text(notify.Event{Severity: notify.Warning, Title: "Report <!channel> & co", Body: "see <https://evil|click> now",
		Link: "https://app.example.com/reports?a=1"})
	for _, bad := range []string{"<!channel>", "<https://evil|click>"} {
		if strings.Contains(got, bad) {
			t.Errorf("unescaped control sequence %q in %q", bad, got)
		}
	}
	if !strings.Contains(got, "&lt;!channel&gt; &amp; co") || !strings.Contains(got, "<https://app.example.com/reports?a=1|Open in Reliabilix>") || !strings.HasPrefix(got, ":warning:") {
		t.Errorf("unexpected text: %q", got)
	}
	if strings.Contains(Text(notify.Event{Severity: notify.Info, Title: "x", Link: "javascript:alert(1)"}), "javascript") {
		t.Error("only http(s) links may be rendered")
	}
}

func TestSendPostsText(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	c := New(safehttp.Options{AllowPrivate: true, AllowHTTP: true})
	if err := c.Send(context.Background(), domain.Target{Address: srv.URL}, domain.Message{Event: notify.Event{Severity: notify.Critical, Title: "Down"}}); err != nil {
		t.Fatal(err)
	}
	if got["text"] != ":rotating_light: *Down*" || got["link_names"] != float64(0) {
		t.Errorf("body = %v", got)
	}
}
