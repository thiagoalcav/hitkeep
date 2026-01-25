package mailer

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/Boostport/mjml-go"

	"hitkeep/internal/config"
	"hitkeep/internal/mailer/drivers"
)

//go:embed templates/*.mjml
var templateFS embed.FS

// Mailer acts as the Manager.
type Mailer struct {
	driver Driver
	conf   *config.Config
}

var ErrMailerDisabled = errors.New("mailer not configured")

type templateContext struct {
	Meta struct {
		Subject string
		Year    int
	}
	Data any
}

// New creates the mailer and resolves the driver based on config.
func New(conf *config.Config) (*Mailer, error) {
	var driver Driver
	var err error

	switch conf.MailDriver {
	case "smtp":
		driver, err = drivers.NewSMTPDriver(conf)
	default:
		err = fmt.Errorf("mail driver '%s' is not implemented. Available drivers: smtp", conf.MailDriver)
	}

	if err != nil {
		return nil, err
	}

	return &Mailer{
		driver: driver,
		conf:   conf,
	}, nil
}

// Send processes a Mailable (renders MJML) and dispatches via the driver.
// Usage: mailer.Send(user.Email, mailables.NewWelcomeEmail(user))
func (m *Mailer) Send(to string, email Mailable) error {
	if m == nil || m.driver == nil {
		return ErrMailerDisabled
	}

	tmpl, err := template.ParseFS(templateFS, "templates/layout.mjml", "templates/"+email.Template())
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	ctx := templateContext{
		Data: email.Data(),
	}
	ctx.Meta.Subject = email.Subject()
	ctx.Meta.Year = time.Now().Year()

	var mjmlBuffer bytes.Buffer
	if err := tmpl.Execute(&mjmlBuffer, ctx); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	htmlContent, err := mjml.ToHTML(context.Background(), mjmlBuffer.String(), mjml.WithMinify(true))
	if err != nil {
		return fmt.Errorf("mjml render error: %w", err)
	}

	// TODO at some point
	return m.driver.Send([]string{to}, email.Subject(), htmlContent, "Please view this email in an HTML client.")
}
