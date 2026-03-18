package mailer

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	htmltpl "html/template"
	"math"
	"strings"
	texttpl "text/template"
	"time"

	"github.com/Boostport/mjml-go"

	"hitkeep/internal/config"
	"hitkeep/internal/mailer/drivers"
)

//go:embed templates/*.mjml templates/*.txt
var templateFS embed.FS

// Mailer acts as the Manager.
type Mailer struct {
	driver Driver
	conf   *config.Config
}

var ErrMailerDisabled = errors.New("mailer not configured")

// templateFuncs contains helpers available to all email templates.
var templateFuncs = htmltpl.FuncMap{
	// percentChange returns a formatted change label like "+12%" or "−3%" or "—".
	"percentChange": func(current, prev int) string {
		if prev == 0 {
			if current == 0 {
				return "—"
			}
			return "+100%"
		}
		pct := (float64(current-prev) / float64(prev)) * 100
		if math.Abs(pct) < 0.05 {
			return "—"
		}
		if pct > 0 {
			return fmt.Sprintf("+%.0f%%", pct)
		}
		return fmt.Sprintf("−%.0f%%", math.Abs(pct))
	},
	// formatDuration formats seconds into "Xm Ys" or "Xs".
	"formatDuration": func(seconds float64) string {
		s := int(seconds)
		if s >= 60 {
			return fmt.Sprintf("%dm %ds", s/60, s%60)
		}
		return fmt.Sprintf("%ds", s)
	},
	// mod2 returns i % 2 for alternating row shading.
	"mod2": func(i int) int { return i % 2 },
	// svgBarChart renders a simple bar chart from a slice of pageview counts and
	// returns an mj-image tag embedding the SVG as a base64 data URI.
	"svgBarChart": func(values []int) htmltpl.HTML {
		const (
			svgW   = 500
			svgH   = 64
			gap    = 2
			radius = 2
			color  = "#3b82f6"
		)
		n := len(values)
		if n == 0 {
			return ""
		}
		maxV := 0
		for _, v := range values {
			if v > maxV {
				maxV = v
			}
		}
		if maxV == 0 {
			return ""
		}
		barW := max((svgW-gap*(n-1))/n, 1)
		var sb strings.Builder
		fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, svgW, svgH)
		for i, v := range values {
			barH := int(math.Round(float64(v) / float64(maxV) * float64(svgH)))
			if barH < 1 && v > 0 {
				barH = 1
			}
			x := i * (barW + gap)
			y := svgH - barH
			fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="%d"/>`,
				x, y, barW, barH, color, radius)
		}
		sb.WriteString(`</svg>`)
		encoded := base64.StdEncoding.EncodeToString([]byte(sb.String()))
		//nolint:gosec // G203: encoded is pure base64 of an SVG built from integer math and constants; no user input enters the string.
		return htmltpl.HTML(`<mj-image src="data:image/svg+xml;base64,` + encoded + `" padding-bottom="20px" width="500px" />`)
	},
}

// textTemplateFuncs is the same set but for text/template (no html.HTML safety wrapper needed).
var textTemplateFuncs = texttpl.FuncMap{
	"percentChange":  templateFuncs["percentChange"],
	"formatDuration": templateFuncs["formatDuration"],
	"mod2":           templateFuncs["mod2"],
}

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

// NewWithDriver creates a Mailer with the specified driver. This is primarily
// useful for testing where a no-op or mock driver is desired.
func NewWithDriver(driver Driver, conf *config.Config) *Mailer {
	return &Mailer{driver: driver, conf: conf}
}

// Send processes a Mailable (renders MJML) and dispatches via the driver.
// Usage: mailer.Send(user.Email, mailables.NewWelcomeEmail(user))
func (m *Mailer) Send(to string, email Mailable) error {
	if m == nil || m.driver == nil {
		return ErrMailerDisabled
	}

	ctx := templateContext{
		Data: email.Data(),
	}
	ctx.Meta.Subject = email.Subject()
	ctx.Meta.Year = time.Now().Year()

	// Render MJML → HTML
	htmlTmpl, err := htmltpl.New("layout.mjml").Funcs(templateFuncs).ParseFS(templateFS, "templates/layout.mjml", "templates/"+email.Template())
	if err != nil {
		return fmt.Errorf("failed to parse html templates: %w", err)
	}

	var mjmlBuffer bytes.Buffer
	if err := htmlTmpl.Execute(&mjmlBuffer, ctx); err != nil {
		return fmt.Errorf("failed to execute html template: %w", err)
	}

	htmlContent, err := mjml.ToHTML(context.Background(), mjmlBuffer.String(), mjml.WithMinify(true))
	if err != nil {
		return fmt.Errorf("mjml render error: %w", err)
	}

	// Render plain-text
	textTemplateName := strings.TrimSuffix(email.Template(), ".mjml") + ".txt"
	textTmpl, err := texttpl.New("layout.txt").Funcs(textTemplateFuncs).ParseFS(templateFS, "templates/layout.txt", "templates/"+textTemplateName)
	if err != nil {
		return fmt.Errorf("failed to parse text templates: %w", err)
	}

	var textBuffer bytes.Buffer
	if err := textTmpl.Execute(&textBuffer, ctx); err != nil {
		return fmt.Errorf("failed to execute text template: %w", err)
	}

	return m.driver.Send([]string{to}, email.Subject(), htmlContent, textBuffer.String())
}
