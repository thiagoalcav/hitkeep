package blocking

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const maxFeedResponseBytes = 10 << 20 // 10 MB

const (
	matomoReferrerSpamURL = "https://raw.githubusercontent.com/matomo-org/referrer-spam-list/master/spammers.txt"
	spamhausDropURL       = "https://www.spamhaus.org/drop/drop.txt"
	spamhausDropV6URL     = "https://www.spamhaus.org/drop/dropv6.txt"
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func FetchSpamFeedData(ctx context.Context, client httpDoer) (SpamFeedData, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	var warnings []string

	referrers, err := fetchMatomoReferrers(ctx, client)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("matomo referrer list: %v", err))
	}

	dropv4, err := fetchSpamhausCIDRs(ctx, client, spamhausDropURL)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("spamhaus drop: %v", err))
	}

	dropv6, err := fetchSpamhausCIDRs(ctx, client, spamhausDropV6URL)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("spamhaus dropv6: %v", err))
	}

	if len(referrers) == 0 && len(dropv4) == 0 && len(dropv6) == 0 {
		return SpamFeedData{}, fmt.Errorf("all spam feeds failed: %s", strings.Join(warnings, "; "))
	}

	for _, w := range warnings {
		slog.Warn("Partial spam feed failure, continuing with available data", "error", w)
	}

	data := SpamFeedData{
		GeneratedAt: time.Now().UTC(),
		Sources: map[string]string{
			"matomo_referrer_spam_list": matomoReferrerSpamURL,
			"spamhaus_drop":             spamhausDropURL,
			"spamhaus_dropv6":           spamhausDropV6URL,
		},
		ReferrerHostDenylist: append([]string(nil), referrers...),
		NetworkDenylist:      append(append([]string(nil), dropv4...), dropv6...),
	}
	data.normalize()
	return data, nil
}

func fetchMatomoReferrers(ctx context.Context, client httpDoer) ([]string, error) {
	body, err := fetchURL(ctx, client, matomoReferrerSpamURL)
	if err != nil {
		return nil, fmt.Errorf("fetch matomo referrer spam list: %w", err)
	}
	defer body.Close()

	var out []string
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, stripWWW(strings.ToLower(line)))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan matomo referrer spam list: %w", err)
	}
	return normalizeStringList(out), nil
}

func fetchSpamhausCIDRs(ctx context.Context, client httpDoer, sourceURL string) ([]string, error) {
	body, err := fetchURL(ctx, client, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch spamhaus cidr list: %w", err)
	}
	defer body.Close()

	var out []string
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		cidr, _, _ := strings.Cut(line, ";")
		cidr = strings.TrimSpace(cidr)
		if cidr != "" {
			out = append(out, cidr)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan spamhaus cidr list: %w", err)
	}
	return normalizeStringList(out), nil
}

func fetchURL(ctx context.Context, client httpDoer, sourceURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.NopCloser(io.LimitReader(resp.Body, maxFeedResponseBytes)), nil
}
