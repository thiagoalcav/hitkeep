package blocking

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

//go:embed default_spam_filter.json
var embeddedSpamDataFS embed.FS

type SpamFeedData struct {
	GeneratedAt          time.Time         `json:"generated_at"`
	Sources              map[string]string `json:"sources"`
	ReferrerHostDenylist []string          `json:"referrer_host_denylist"`
	NetworkDenylist      []string          `json:"network_denylist"`
}

func LoadEmbeddedSpamFeedData() (SpamFeedData, error) {
	raw, err := embeddedSpamDataFS.ReadFile("default_spam_filter.json")
	if err != nil {
		return SpamFeedData{}, fmt.Errorf("read embedded spam data: %w", err)
	}
	return decodeSpamFeedData(raw)
}

func LoadSpamFeedData(path string) (SpamFeedData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SpamFeedData{}, err
	}
	return decodeSpamFeedData(raw)
}

func SaveSpamFeedData(path string, data SpamFeedData) error {
	data.normalize()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create spam filter cache dir: %w", err)
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal spam feed data: %w", err)
	}
	raw = append(raw, '\n')

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write spam feed data: %w", err)
	}
	return nil
}

func decodeSpamFeedData(raw []byte) (SpamFeedData, error) {
	var data SpamFeedData
	if err := json.Unmarshal(raw, &data); err != nil {
		return SpamFeedData{}, fmt.Errorf("decode spam feed data: %w", err)
	}
	data.normalize()
	return data, nil
}

func (d *SpamFeedData) normalize() {
	if d.Sources == nil {
		d.Sources = make(map[string]string)
	}
	d.ReferrerHostDenylist = normalizeStringList(d.ReferrerHostDenylist)
	d.NetworkDenylist = normalizeStringList(d.NetworkDenylist)
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}
