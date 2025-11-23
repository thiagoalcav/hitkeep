package mailer

// Driver represents the underlying transport mechanism (SMTP, Vendor, etc.)
type Driver interface {
	// Send transmits the constructed message.
	Send(to []string, subject string, htmlBody string, textBody string) error
	// Close cleans up connections if necessary (e.g., SMTP pool).
	Close() error
}
