// Package slack delivers events to a Slack incoming webhook. The URL is a bearer credential, so the platform seals
// it at rest, pins the host to hooks.slack.com and never echoes it back.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/safehttp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

const host = "hooks.slack.com"

type Channel struct {
	Client *http.Client
	Opts   safehttp.Options
}

func New(o safehttp.Options) *Channel { return &Channel{Opts: o, Client: safehttp.NewClient(o)} }

func (*Channel) Kind() domain.Kind { return domain.KindSlack }

func (c *Channel) Prepare(address string) (string, string, error) {
	u, err := safehttp.ParseURL(address, c.Opts)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(u.Hostname(), host) || u.Port() != "" || !strings.HasPrefix(u.Path, "/services/") || u.RawQuery != "" {
		return "", "", fmt.Errorf("%w: expected a Slack incoming webhook URL (https://%s/services/...)", domain.ErrInvalid, host)
	}
	return u.String(), u.Hostname(), nil
}

// escape applies Slack's mrkdwn control-character escaping, so event text can never inject mentions or links.
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

var icon = map[notify.Severity]string{notify.Info: ":information_source:", notify.Warning: ":warning:", notify.Critical: ":rotating_light:"}

// Text renders an event as a Slack message.
func Text(e notify.Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s *%s*", icon[e.Severity], escape(e.Title))
	if e.Body != "" {
		b.WriteString("\n" + escape(e.Body))
	}
	if u, err := url.Parse(e.Link); err == nil && e.Link != "" && (u.Scheme == "https" || u.Scheme == "http") {
		fmt.Fprintf(&b, "\n<%s|Open in Reliabilix>", strings.NewReplacer("|", "%7C", ">", "%3E", "<", "%3C").Replace(e.Link))
	}
	return b.String()
}

func (c *Channel) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	body, err := json.Marshal(map[string]any{"text": Text(m.Event), "link_names": 0, "unfurl_links": false})
	if err != nil {
		return domain.Permanent(err)
	}
	return safehttp.PostJSON(ctx, c.Client, t.Address, map[string]string{"User-Agent": "Reliabilix-Notifications/1"}, body)
}
