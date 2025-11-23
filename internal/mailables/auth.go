package mailables

import "hitkeep/internal/mailer"

// PasswordReset implements the mailer.Mailable interface
type PasswordReset struct {
	Link string
}

func NewPasswordReset(link string) mailer.Mailable {
	return &PasswordReset{Link: link}
}

func (m *PasswordReset) Subject() string {
	return "Reset your HitKeep Password"
}

func (m *PasswordReset) Template() string {
	return "password_reset.mjml"
}

func (m *PasswordReset) Data() any {
	return struct {
		Link string
	}{
		Link: m.Link,
	}
}
