package application

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/i18n"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

// InviteMailer delivers one invitation e-mail. Available is false while the platform has no mail transport.
type InviteMailer interface {
	Available(ctx context.Context) bool
	SendInvitation(ctx context.Context, to, subject, body, footer, link string) error
}

// InviteLink is where the invitee goes: the web app's /invite page, which signs them in and accepts the token.
func InviteLink(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/invite?token=" + url.QueryEscape(token)
}

// SendInvitation e-mails the invitee the link, in the inviter's language. It reports whether a message went out; a failure
// is not an error for the caller, the token is still shown to the inviter. The token travels only in this message (and the
// link), it is not stored anywhere.
func (s Service) SendInvitation(ctx context.Context, tenantID, lang string, inv domain.Invitation, token string) bool {
	if s.Mail == nil || s.BaseURL == "" || !s.Mail.Available(ctx) {
		return false
	}
	org, err := s.Repo.Organization(ctx, tenantID)
	if err != nil {
		return false
	}
	days := s.cfg().Int(ctx, tenantID, settings.KeyInviteExpiryDays)
	if days <= 0 {
		days = 7
	}
	subject := i18n.T(lang, "invite.subject", "org", org.Name)
	body := i18n.T(lang, "invite.body", "org", org.Name, "role", string(inv.Role), "days", strconv.FormatInt(days, 10))
	return s.Mail.SendInvitation(ctx, inv.Email, subject, body, i18n.T(lang, "invite.footer"), InviteLink(s.BaseURL, token)) == nil
}
