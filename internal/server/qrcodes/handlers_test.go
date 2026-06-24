package qrcodes

import (
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func TestBuildDestinationURLAppliesCampaignParametersAndQRAttribution(t *testing.T) {
	t.Parallel()

	qrID := uuid.MustParse("0f0bbce0-3b6f-4976-9392-95e0a2a7e87f")

	got, err := buildDestinationURL(api.QRCode{
		ID:             qrID,
		DestinationURL: "https://example.com/signup?existing=1&hk_qr=old",
		UTMSource:      "poster",
		UTMMedium:      "print",
		UTMCampaign:    "spring launch",
		UTMContent:     "front window",
		CustomParams:   map[string]string{"region": "berlin", "empty": ""},
	})
	if err != nil {
		t.Fatalf("build destination: %v", err)
	}

	want := "https://example.com/signup?existing=1&hk_qr=0f0bbce0-3b6f-4976-9392-95e0a2a7e87f&region=berlin&utm_campaign=spring+launch&utm_content=front+window&utm_medium=print&utm_source=poster"
	if got != want {
		t.Fatalf("destination mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestBuildDestinationURLRejectsUnsafeDestinationURLs(t *testing.T) {
	t.Parallel()

	qrID := uuid.MustParse("0f0bbce0-3b6f-4976-9392-95e0a2a7e87f")
	tests := []struct {
		name           string
		destinationURL string
	}{
		{name: "empty", destinationURL: ""},
		{name: "relative path", destinationURL: "/signup"},
		{name: "protocol relative", destinationURL: "//example.com/signup"},
		{name: "unsupported scheme", destinationURL: "ftp://example.com/signup"},
		{name: "javascript scheme", destinationURL: "javascript:alert(1)"},
		{name: "missing host", destinationURL: "https:///signup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := buildDestinationURL(api.QRCode{
				ID:             qrID,
				DestinationURL: tt.destinationURL,
			})
			if err == nil {
				t.Fatalf("expected error for %q, got %q", tt.destinationURL, got)
			}
		})
	}
}
