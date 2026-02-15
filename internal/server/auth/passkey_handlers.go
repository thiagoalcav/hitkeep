package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
	"hitkeep/internal/security"
)

const (
	passkeyLoginChallengeTTL = 5 * time.Minute
	maxAuthPayloadBytes      = 1 << 20
)

type passkeyLoginStartResponse struct {
	ChallengeToken string                     `json:"challenge_token"`
	PublicKey      passkeyLoginRequestOptions `json:"publicKey"`
}

type passkeyLoginRequestOptions struct {
	Challenge        string `json:"challenge"`
	RPID             string `json:"rpId"`
	Timeout          int    `json:"timeout"`
	UserVerification string `json:"userVerification"`
}

type passkeyLoginFinishRequest struct {
	ChallengeToken    string `json:"challenge_token"`
	CredentialID      string `json:"credential_id"`
	ClientDataJSON    string `json:"client_data_json"`
	AuthenticatorData string `json:"authenticator_data"`
	Signature         string `json:"signature"`
	RememberMe        bool   `json:"remember_me"`
}

type webAuthnClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func (h *handler) handlePasskeyLoginStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Accept optional empty JSON payload to keep the endpoint extensible.
		if !decodeAuthJSON(w, r, &struct{}{}, true) {
			return
		}

		challenge, err := security.GenerateRandomChallenge(32)
		if err != nil {
			slog.Error("Failed to generate passkey login challenge", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(passkeyLoginChallengeTTL)
		challengeID, err := h.ctx.Store.CreatePasskeyLoginChallenge(r.Context(), challenge, database.CreateLoginChallengeInput{
			Flow: "passwordless",
		}, expiresAt)
		if err != nil {
			slog.Error("Failed to create passkey login challenge", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resp := passkeyLoginStartResponse{
			ChallengeToken: challengeID.String(),
			PublicKey:      *h.newPasskeyLoginRequestOptions(r, challenge),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode passkey login start response", "error", err)
		}
	}
}

func (h *handler) handlePasskeyLoginFinish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req passkeyLoginFinishRequest
		if !decodeAuthJSON(w, r, &req, false) {
			return
		}
		req.CredentialID = strings.TrimSpace(req.CredentialID)
		if req.CredentialID == "" {
			http.Error(w, "Credential ID is required", http.StatusBadRequest)
			return
		}

		challengeID, err := uuid.Parse(strings.TrimSpace(req.ChallengeToken))
		if err != nil {
			http.Error(w, "Invalid challenge token", http.StatusBadRequest)
			return
		}

		challenge, found, err := h.ctx.Store.GetPasskeyLoginChallenge(r.Context(), challengeID)
		if err != nil {
			slog.Error("Failed to load passkey login challenge", "error", err, "challenge_id", challengeID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "Passkey login challenge not found", http.StatusConflict)
			return
		}
		// Single use challenge.
		if err := h.ctx.Store.DeletePasskeyLoginChallenge(r.Context(), challengeID); err != nil {
			slog.Warn("Failed to delete passkey login challenge", "error", err, "challenge_id", challengeID)
		}
		if time.Now().UTC().After(challenge.ExpiresAt.UTC()) {
			http.Error(w, "Passkey login challenge expired", http.StatusGone)
			return
		}

		clientDataBytes, err := decodeBase64URL(req.ClientDataJSON)
		if err != nil {
			http.Error(w, "Invalid clientDataJSON", http.StatusBadRequest)
			return
		}
		authenticatorData, err := decodeBase64URL(req.AuthenticatorData)
		if err != nil {
			http.Error(w, "Invalid authenticatorData", http.StatusBadRequest)
			return
		}
		signature, err := decodeBase64URL(req.Signature)
		if err != nil {
			http.Error(w, "Invalid signature", http.StatusBadRequest)
			return
		}

		var clientData webAuthnClientData
		if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
			http.Error(w, "Invalid clientDataJSON", http.StatusBadRequest)
			return
		}
		if clientData.Type != "webauthn.get" {
			http.Error(w, "Invalid passkey login type", http.StatusBadRequest)
			return
		}
		if clientData.Challenge != challenge.Challenge {
			http.Error(w, "Passkey login challenge mismatch", http.StatusForbidden)
			return
		}

		rpID, expectedOrigin := h.passkeyRelyingParty(r)
		if normalizeOrigin(clientData.Origin) != normalizeOrigin(expectedOrigin) {
			http.Error(w, "Passkey login origin mismatch", http.StatusForbidden)
			return
		}

		passkey, err := h.ctx.Store.GetPasskeyByCredentialID(r.Context(), req.CredentialID)
		if err != nil {
			slog.Error("Failed to load passkey credential", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if passkey == nil || strings.TrimSpace(passkey.PublicKey) == "" {
			http.Error(w, "Invalid passkey credential", http.StatusUnauthorized)
			return
		}
		if strings.TrimSpace(challenge.Flow) == "mfa" && !challenge.HasUserID {
			http.Error(w, "Invalid MFA challenge context", http.StatusForbidden)
			return
		}
		if challenge.HasUserID && challenge.UserID != passkey.UserID {
			http.Error(w, "Passkey does not match MFA challenge user", http.StatusForbidden)
			return
		}

		newSignCount, err := verifyPasskeyAssertion(passkey.PublicKey, rpID, clientDataBytes, authenticatorData, signature, passkey.SignCount)
		if err != nil {
			slog.Warn("Passkey assertion verification failed", "error", err, "credential_id", req.CredentialID)
			http.Error(w, "Invalid passkey assertion", http.StatusUnauthorized)
			return
		}

		shouldUpdateSignCount := newSignCount > passkey.SignCount
		if shouldUpdateSignCount {
			if err := h.ctx.Store.UpdatePasskeySignCount(r.Context(), passkey.ID, newSignCount); err != nil {
				slog.Error("Failed to update passkey sign count", "error", err, "passkey_id", passkey.ID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		rememberMe := req.RememberMe
		if strings.TrimSpace(challenge.Flow) == "mfa" {
			rememberMe = challenge.RememberMe
		}
		if err := h.issueLoginSession(r.Context(), w, passkey.UserID, rememberMe); err != nil {
			slog.Error("Failed to issue login session after passkey verification", "error", err, "user_id", passkey.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode passkey login response", "error", err, "user_id", passkey.UserID)
		}
	}
}

func (h *handler) newPasskeyLoginRequestOptions(r *http.Request, challenge string) *passkeyLoginRequestOptions {
	rpID, _ := h.passkeyRelyingParty(r)
	return &passkeyLoginRequestOptions{
		Challenge:        challenge,
		RPID:             rpID,
		Timeout:          60_000,
		UserVerification: "preferred",
	}
}

func verifyPasskeyAssertion(publicKeyB64 string, rpID string, clientDataJSON []byte, authenticatorData []byte, signature []byte, previousSignCount uint32) (uint32, error) {
	publicKeyDER, err := decodeBase64URL(publicKeyB64)
	if err != nil {
		return 0, fmt.Errorf("invalid stored public key: %w", err)
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return 0, fmt.Errorf("invalid stored public key: %w", err)
	}

	if len(authenticatorData) < 37 {
		return 0, fmt.Errorf("authenticator data too short")
	}

	expectedRPIDHash := sha256.Sum256([]byte(rpID))
	if !equalBytes(authenticatorData[:32], expectedRPIDHash[:]) {
		return 0, fmt.Errorf("rp id hash mismatch")
	}

	flags := authenticatorData[32]
	const userPresentBit = byte(0x01)
	if flags&userPresentBit == 0 {
		return 0, fmt.Errorf("user present flag is not set")
	}

	signCount := binary.BigEndian.Uint32(authenticatorData[33:37])
	if previousSignCount > 0 && signCount > 0 && signCount <= previousSignCount {
		return 0, fmt.Errorf("sign counter replay detected")
	}

	clientDataHash := sha256.Sum256(clientDataJSON)
	signed := make([]byte, 0, len(authenticatorData)+len(clientDataHash))
	signed = append(signed, authenticatorData...)
	signed = append(signed, clientDataHash[:]...)
	digest := sha256.Sum256(signed)

	switch key := publicKey.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(key, digest[:], signature) {
			return 0, fmt.Errorf("ecdsa signature verification failed")
		}
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
			return 0, fmt.Errorf("rsa signature verification failed")
		}
	default:
		return 0, fmt.Errorf("unsupported passkey public key type")
	}

	return signCount, nil
}

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, dest any, allowEmpty bool) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAuthPayloadBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		if allowEmpty && err == io.EOF {
			return true
		}
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return false
	}
	return true
}

func decodeBase64URL(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, io.EOF
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

func normalizeOrigin(origin string) string {
	return strings.TrimRight(strings.TrimSpace(origin), "/")
}

func (h *handler) passkeyRelyingParty(r *http.Request) (rpID string, origin string) {
	if parsed, err := url.Parse(h.ctx.Config.PublicURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Hostname(), parsed.Scheme + "://" + parsed.Host
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
		scheme = forwardedProto
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "localhost:8080"
	}
	return hostNameOnly(host), scheme + "://" + host
}

func hostNameOnly(host string) string {
	if parsed, err := url.Parse("http://" + host); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	if name, _, err := net.SplitHostPort(host); err == nil && name != "" {
		return name
	}
	return host
}

func equalBytes(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
