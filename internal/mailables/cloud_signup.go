//go:build billing

package mailables

import "hitkeep/internal/mailer"

type EmailVerification struct {
	Link     string
	TeamName string
}

func NewEmailVerification(link, teamName string) mailer.Mailable {
	return &EmailVerification{Link: link, TeamName: teamName}
}

func (m *EmailVerification) Subject() string {
	return "Verify your email — HitKeep"
}

func (m *EmailVerification) Template() string {
	return "email_verification.mjml"
}

func (m *EmailVerification) Data() any {
	return struct {
		Link     string
		TeamName string
	}{
		Link:     m.Link,
		TeamName: m.TeamName,
	}
}
