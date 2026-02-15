package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- RFC 6238 TOTP compatibility with standard authenticator apps.
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	totpDigits      = 6
	totpPeriod      = 30
	totpSecretBytes = 20
)

var base32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, totpSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate totp secret: %w", err)
	}
	return base32NoPadding.EncodeToString(buf), nil
}

func BuildOTPAuthURL(issuer string, account string, secret string) string {
	label := strings.TrimSpace(account)
	if label == "" {
		label = "account"
	}
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		issuer = "HitKeep"
	}

	encodedLabel := url.PathEscape(fmt.Sprintf("%s:%s", issuer, label))
	values := url.Values{
		"secret":    {strings.TrimSpace(secret)},
		"issuer":    {issuer},
		"algorithm": {"SHA1"},
		"digits":    {strconv.Itoa(totpDigits)},
		"period":    {strconv.Itoa(totpPeriod)},
	}

	return "otpauth://totp/" + encodedLabel + "?" + values.Encode()
}

func ValidateTOTPCode(secret string, code string, now time.Time) bool {
	return ValidateTOTPCodeWithWindow(secret, code, now, 1, 1)
}

func ValidateTOTPCodeStrict(secret string, code string, now time.Time) bool {
	return ValidateTOTPCodeWithWindow(secret, code, now, 0, 0)
}

func ValidateTOTPCodeWithWindow(secret string, code string, now time.Time, pastWindow int, futureWindow int) bool {
	normalizedCode := normalizeTOTPCode(code)
	if normalizedCode == "" {
		return false
	}

	normalizedSecret := strings.TrimSpace(strings.ToUpper(secret))
	key, err := base32NoPadding.DecodeString(normalizedSecret)
	if err != nil || len(key) == 0 {
		return false
	}

	if pastWindow < 0 {
		pastWindow = 0
	}
	if futureWindow < 0 {
		futureWindow = 0
	}

	counter := now.UTC().Unix() / totpPeriod
	for delta := int64(-pastWindow); delta <= int64(futureWindow); delta++ {
		windowCounter := counter + delta
		if windowCounter < 0 {
			continue
		}
		expected := generateHOTPCode(key, windowCounter)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(normalizedCode)) == 1 {
			return true
		}
	}
	return false
}

func GenerateCurrentTOTPCode(secret string, now time.Time) (string, error) {
	normalizedSecret := strings.TrimSpace(strings.ToUpper(secret))
	key, err := base32NoPadding.DecodeString(normalizedSecret)
	if err != nil || len(key) == 0 {
		return "", fmt.Errorf("invalid totp secret")
	}
	counter := now.UTC().Unix() / totpPeriod
	if counter < 0 {
		return "", fmt.Errorf("invalid time for totp generation")
	}
	return generateHOTPCode(key, counter), nil
}

func GenerateRandomChallenge(size int) (string, error) {
	if size <= 0 {
		size = 32
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("could not generate random challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeTOTPCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return ""
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return code
}

func generateHOTPCode(key []byte, counter int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter)) // #nosec G115 -- callers guarantee non-negative TOTP counters.

	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	otp := binaryCode % 1_000_000

	return fmt.Sprintf("%06d", otp)
}
