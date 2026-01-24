package mailables

import "hitkeep/internal/mailer"

// UserInvite implements the mailer.Mailable interface
type UserInvite struct {
	Link     string
	SiteName string
	Inviter  string
}

func NewUserInvite(link, siteName, inviter string) mailer.Mailable {
	return &UserInvite{
		Link:     link,
		SiteName: siteName,
		Inviter:  inviter,
	}
}

func (m *UserInvite) Subject() string {
	return "You've been invited to join " + m.SiteName + " on HitKeep"
}

func (m *UserInvite) Template() string {
	return "user_invite.mjml"
}

func (m *UserInvite) Data() any {
	return struct {
		Link     string
		SiteName string
		Inviter  string
	}{
		Link:     m.Link,
		SiteName: m.SiteName,
		Inviter:  m.Inviter,
	}
}
