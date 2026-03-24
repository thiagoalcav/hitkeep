package aianalytics

import "strings"

type BotIdentity struct {
	Name   string
	Family string
}

type botMatcher struct {
	token    string
	identity BotIdentity
}

//nolint:gosec // These are public user-agent tokens used for bot classification, not credentials.
var botMatchers = []botMatcher{
	{token: "chatgpt-user", identity: BotIdentity{Name: "ChatGPT-User", Family: "OpenAI"}},
	{token: "gptbot", identity: BotIdentity{Name: "GPTBot", Family: "OpenAI"}},
	{token: "claudebot", identity: BotIdentity{Name: "ClaudeBot", Family: "Anthropic"}},
	{token: "claude-web", identity: BotIdentity{Name: "Claude-Web", Family: "Anthropic"}},
	{token: "perplexitybot", identity: BotIdentity{Name: "PerplexityBot", Family: "Perplexity"}},
	{token: "google-extended", identity: BotIdentity{Name: "Google-Extended", Family: "Google"}},
	{token: "googleother", identity: BotIdentity{Name: "GoogleOther", Family: "Google"}},
	{token: "google-safety", identity: BotIdentity{Name: "Google-Safety", Family: "Google"}},
	{token: "applebot-extended", identity: BotIdentity{Name: "Applebot-Extended", Family: "Apple"}},
	{token: "bytespider", identity: BotIdentity{Name: "Bytespider", Family: "ByteDance"}},
	{token: "ccbot", identity: BotIdentity{Name: "CCBot", Family: "Common Crawl"}},
	{token: "meta-externalagent", identity: BotIdentity{Name: "Meta-ExternalAgent", Family: "Meta"}},
	{token: "meta-externalfetcher", identity: BotIdentity{Name: "Meta-ExternalFetcher", Family: "Meta"}},
	{token: "amazonbot", identity: BotIdentity{Name: "Amazonbot", Family: "Amazon"}},
	{token: "cohere-ai", identity: BotIdentity{Name: "Cohere", Family: "Cohere"}},
	{token: "youbot", identity: BotIdentity{Name: "YouBot", Family: "You.com"}},
	{token: "ai2bot", identity: BotIdentity{Name: "AI2Bot", Family: "AI2"}},
	{token: "diffbot", identity: BotIdentity{Name: "Diffbot", Family: "Diffbot"}},
	{token: "timpibot", identity: BotIdentity{Name: "Timpibot", Family: "Timpi"}},
	{token: "imagesiftbot", identity: BotIdentity{Name: "ImagesiftBot", Family: "Imagesift"}},
	{token: "deepseekbot", identity: BotIdentity{Name: "DeepSeek", Family: "DeepSeek"}},
	{token: "petalbot", identity: BotIdentity{Name: "PetalBot", Family: "Petal"}},
}

func ClassifyBot(userAgent string) *BotIdentity {
	normalized := strings.ToLower(strings.TrimSpace(userAgent))
	if normalized == "" {
		return nil
	}

	for _, matcher := range botMatchers {
		if strings.Contains(normalized, matcher.token) {
			identity := matcher.identity
			return &identity
		}
	}

	return nil
}

func ClassifyResourceType(contentType string) string {
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = normalized[:idx]
	}

	switch {
	case normalized == "", normalized == "text/html", normalized == "application/xhtml+xml":
		return "html"
	case strings.HasPrefix(normalized, "image/"):
		return "image"
	case normalized == "application/pdf",
		normalized == "application/msword",
		normalized == "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		normalized == "application/vnd.ms-powerpoint",
		normalized == "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		normalized == "application/vnd.ms-excel",
		normalized == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		normalized == "text/plain",
		normalized == "text/markdown":
		return "document"
	default:
		return "other"
	}
}
