package aianalytics

import "testing"

func TestClassifyBot(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		wantName  string
		wantGroup string
	}{
		{
			name:      "matches GPTBot",
			userAgent: "Mozilla/5.0 (compatible; GPTBot/1.0; +https://openai.com/gptbot)",
			wantName:  "GPTBot",
			wantGroup: "OpenAI",
		},
		{
			name:      "matches ClaudeBot",
			userAgent: "Mozilla/5.0 (compatible; ClaudeBot/1.0; +https://anthropic.com/claudebot)",
			wantName:  "ClaudeBot",
			wantGroup: "Anthropic",
		},
		{
			name:      "matches DeepSeekBot",
			userAgent: "Mozilla/5.0 (compatible; DeepSeekBot/1.0; +https://deepseek.com/bot)",
			wantName:  "DeepSeek",
			wantGroup: "DeepSeek",
		},
		{
			name:      "does not overmatch DeepSeekBrowser",
			userAgent: "Mozilla/5.0 (compatible; DeepSeekBrowser/1.0; +https://example.com/bot)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			identity := ClassifyBot(tc.userAgent)
			if tc.wantName == "" {
				if identity != nil {
					t.Fatalf("expected nil identity, got %+v", *identity)
				}
				return
			}
			if identity == nil {
				t.Fatal("expected identity, got nil")
			}
			if identity.Name != tc.wantName || identity.Family != tc.wantGroup {
				t.Fatalf("unexpected identity: got %+v", *identity)
			}
		})
	}
}

func TestClassifyResourceType(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{contentType: "", want: "html"},
		{contentType: "text/html; charset=utf-8", want: "html"},
		{contentType: "application/pdf", want: "document"},
		{contentType: "image/png", want: "image"},
		{contentType: "application/json", want: "other"},
	}

	for _, tc := range tests {
		if got := ClassifyResourceType(tc.contentType); got != tc.want {
			t.Fatalf("ClassifyResourceType(%q) = %q, want %q", tc.contentType, got, tc.want)
		}
	}
}
