package qrcodes

import (
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func TestBuildDestinationURLAppliesCampaignParametersAndQRAttribution(t *testing.T) {
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
