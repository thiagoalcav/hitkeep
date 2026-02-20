package drivers

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"hitkeep/internal/config"
)

type SMTPDriver struct {
	client *mail.Client
	from   string
	name   string
}

func NewSMTPDriver(conf *config.Config) (*SMTPDriver, error) {
	opts := []mail.Option{
		mail.WithPort(conf.MailPort),
		mail.WithTimeout(10 * time.Second),
		mail.WithHELO(conf.PublicURL),
	}

	if conf.MailUsername != "" {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(conf.MailUsername),
			mail.WithPassword(conf.MailPassword),
		)
	}

	switch conf.MailEncryption {
	case "ssl":
		opts = append(opts, mail.WithSSL())
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	case "tls":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	default:
		opts = append(opts, mail.WithTLSPolicy(mail.DefaultTLSPolicy))
	}

	if conf.MailInsecureSkipVerify {
		//nolint:gosec // user asked to
		opts = append(opts, mail.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	}

	client, err := mail.NewClient(conf.MailHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create smtp client: %w", err)
	}

	return &SMTPDriver{
		client: client,
		from:   conf.MailFromAddress,
		name:   conf.MailFromName,
	}, nil
}

func (s *SMTPDriver) Send(to []string, subject string, htmlBody string, textBody string) error {
	msg := mail.NewMsg()
	if err := msg.FromFormat(s.name, s.from); err != nil {
		return err
	}
	if err := msg.To(to...); err != nil {
		return err
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, textBody)
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)

	return s.client.DialAndSend(msg)
}

func (s *SMTPDriver) Close() error {
	return s.client.Close()
}
