package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/security"
	"hitkeep/internal/server/shared"
)

const (
	maxSecurityPayloadBytes = 1 << 20
	totpSetupTTL            = 10 * time.Minute
	passkeyChallengeTTL     = 5 * time.Minute
)

type passkeyRegistrationStartRequest struct {
	Name string `json:"name"`
}

type passkeyRegistrationStartResponse struct {
	PublicKey passkeyCreationOptions `json:"publicKey"`
}

type passkeyCreationOptions struct {
	Challenge              string                        `json:"challenge"`
	RP                     passkeyRPEntity               `json:"rp"`
	User                   passkeyUserEntity             `json:"user"`
	PubKeyCredParams       []passkeyCredentialParameter  `json:"pubKeyCredParams"`
	Timeout                int                           `json:"timeout"`
	Attestation            string                        `json:"attestation"`
	AuthenticatorSelection passkeyAuthenticatorSelection `json:"authenticatorSelection"`
}

type passkeyRPEntity struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type passkeyUserEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type passkeyCredentialParameter struct {
	Type string `json:"type"`
	Alg  int    `json:"alg"`
}

type passkeyAuthenticatorSelection struct {
	ResidentKey      string `json:"residentKey"`
	UserVerification string `json:"userVerification"`
}

type verifyTOTPRequest struct {
	Code string `json:"code"`
}

type disableTOTPRequest struct {
	Code string `json:"code"`
}

type passkeyRegistrationFinishRequest struct {
	Name           string   `json:"name"`
	CredentialID   string   `json:"credential_id"`
	ClientDataJSON string   `json:"client_data_json"`
	PublicKey      string   `json:"public_key"`
	Transports     []string `json:"transports"`
}

type webAuthnClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func (h *handler) handleGetUserSecurityStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		status, err := h.getUserSecurityStatus(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user security status", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			slog.Error("Failed to encode user security status", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleStartTOTPSetup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := h.ctx.Store.GetUserByID(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user for totp setup", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		secret, err := security.GenerateTOTPSecret()
		if err != nil {
			slog.Error("Failed to generate totp secret", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(totpSetupTTL)
		if err := h.ctx.Store.CreatePendingTOTPSetup(r.Context(), userID, secret, expiresAt); err != nil {
			slog.Error("Failed to create pending totp setup", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		issuer := h.totpIssuer()
		resp := api.UserTOTPSetup{
			Secret:     secret,
			OTPAuthURL: security.BuildOTPAuthURL(issuer, user.Email, secret),
			ExpiresAt:  expiresAt,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode pending totp setup", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleVerifyTOTPSetup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req verifyTOTPRequest
		if !decodeSecurityJSON(w, r, &req, false) {
			return
		}

		secret, expiresAt, found, err := h.ctx.Store.GetPendingTOTPSetup(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load pending totp setup", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "No TOTP setup is pending", http.StatusConflict)
			return
		}
		if time.Now().UTC().After(expiresAt.UTC()) {
			_ = h.ctx.Store.DeletePendingTOTPSetup(r.Context(), userID)
			http.Error(w, "TOTP setup expired", http.StatusGone)
			return
		}
		if !security.ValidateTOTPCode(secret, req.Code, time.Now().UTC()) {
			http.Error(w, "Invalid TOTP code", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.EnableUserTOTP(r.Context(), userID, secret); err != nil {
			slog.Error("Failed to enable totp", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		status, err := h.getUserSecurityStatus(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user security status after enabling totp", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			slog.Error("Failed to encode security status after enabling totp", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleDisableTOTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req disableTOTPRequest
		if !decodeSecurityJSON(w, r, &req, false) {
			return
		}

		secret, found, err := h.ctx.Store.GetUserTOTPSecret(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to get user totp secret", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "TOTP is not enabled", http.StatusNotFound)
			return
		}
		if !security.ValidateTOTPCode(secret, req.Code, time.Now().UTC()) {
			http.Error(w, "Invalid TOTP code", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.DisableUserTOTP(r.Context(), userID); err != nil {
			slog.Error("Failed to disable user totp", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		status, err := h.getUserSecurityStatus(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user security status after disabling totp", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			slog.Error("Failed to encode security status after disabling totp", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleStartPasskeyRegistration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := h.ctx.Store.GetUserByID(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user for passkey registration", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		var req passkeyRegistrationStartRequest
		if !decodeSecurityJSON(w, r, &req, true) {
			return
		}

		challenge, err := security.GenerateRandomChallenge(32)
		if err != nil {
			slog.Error("Failed to generate passkey challenge", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().UTC().Add(passkeyChallengeTTL)
		if err := h.ctx.Store.CreatePasskeyChallenge(r.Context(), userID, challenge, strings.TrimSpace(req.Name), expiresAt); err != nil {
			slog.Error("Failed to store passkey challenge", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		rpID, _ := h.passkeyRelyingParty(r)
		userIDB64 := base64.RawURLEncoding.EncodeToString(userID[:])
		resp := passkeyRegistrationStartResponse{
			PublicKey: passkeyCreationOptions{
				Challenge: challenge,
				RP: passkeyRPEntity{
					Name: "HitKeep",
					ID:   rpID,
				},
				User: passkeyUserEntity{
					ID:          userIDB64,
					Name:        user.Email,
					DisplayName: displayNameFromEmail(user.Email),
				},
				PubKeyCredParams: []passkeyCredentialParameter{
					{Type: "public-key", Alg: -7},
					{Type: "public-key", Alg: -257},
				},
				Timeout:     60_000,
				Attestation: "none",
				AuthenticatorSelection: passkeyAuthenticatorSelection{
					ResidentKey:      "required",
					UserVerification: "required",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode passkey registration start response", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleFinishPasskeyRegistration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req passkeyRegistrationFinishRequest
		if !decodeSecurityJSON(w, r, &req, false) {
			return
		}

		req.CredentialID = strings.TrimSpace(req.CredentialID)
		if req.CredentialID == "" {
			http.Error(w, "Credential ID is required", http.StatusBadRequest)
			return
		}
		req.PublicKey = strings.TrimSpace(req.PublicKey)
		if req.PublicKey == "" {
			http.Error(w, "Public key is required for passkey registration", http.StatusBadRequest)
			return
		}

		challenge, requestedName, expiresAt, found, err := h.ctx.Store.GetPasskeyChallenge(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load passkey challenge", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "No passkey registration is pending", http.StatusConflict)
			return
		}
		if time.Now().UTC().After(expiresAt.UTC()) {
			_ = h.ctx.Store.DeletePasskeyChallenge(r.Context(), userID)
			http.Error(w, "Passkey registration challenge expired", http.StatusGone)
			return
		}

		clientDataBytes, err := decodeBase64URL(req.ClientDataJSON)
		if err != nil {
			http.Error(w, "Invalid clientDataJSON", http.StatusBadRequest)
			return
		}

		var clientData webAuthnClientData
		if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
			http.Error(w, "Invalid clientDataJSON", http.StatusBadRequest)
			return
		}
		if clientData.Type != "webauthn.create" {
			http.Error(w, "Invalid passkey registration type", http.StatusBadRequest)
			return
		}
		if clientData.Challenge != challenge {
			http.Error(w, "Passkey registration challenge mismatch", http.StatusForbidden)
			return
		}

		_, expectedOrigin := h.passkeyRelyingParty(r)
		if normalizeOrigin(clientData.Origin) != normalizeOrigin(expectedOrigin) {
			http.Error(w, "Passkey registration origin mismatch", http.StatusForbidden)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = strings.TrimSpace(requestedName)
		}
		if name == "" {
			name = "Passkey " + time.Now().Format("2006-01-02")
		}

		if _, err := h.ctx.Store.CreateUserPasskey(r.Context(), userID, name, req.CredentialID, req.PublicKey, req.Transports); err != nil {
			slog.Error("Failed to create user passkey", "error", err, "user_id", userID)
			http.Error(w, "Failed to save passkey", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.DeletePasskeyChallenge(r.Context(), userID); err != nil {
			slog.Warn("Failed to delete passkey challenge after completion", "error", err, "user_id", userID)
		}

		status, err := h.getUserSecurityStatus(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user security status after passkey registration", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			slog.Error("Failed to encode security status after passkey registration", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleDeleteUserPasskey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		passkeyID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "Invalid passkey ID", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.DeleteUserPasskey(r.Context(), userID, passkeyID); err != nil {
			if strings.Contains(err.Error(), "no rows affected") {
				http.Error(w, "Passkey not found", http.StatusNotFound)
				return
			}
			slog.Error("Failed to delete user passkey", "error", err, "user_id", userID, "passkey_id", passkeyID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *handler) getUserSecurityStatus(ctx context.Context, userID uuid.UUID) (api.UserSecurityStatus, error) {
	totpEnabled, err := h.ctx.Store.HasEnabledTOTP(ctx, userID)
	if err != nil {
		return api.UserSecurityStatus{}, err
	}
	totpPending, err := h.ctx.Store.HasPendingTOTPSetup(ctx, userID)
	if err != nil {
		return api.UserSecurityStatus{}, err
	}
	passkeys, err := h.ctx.Store.ListUserPasskeys(ctx, userID)
	if err != nil {
		return api.UserSecurityStatus{}, err
	}
	return api.UserSecurityStatus{
		TOTPEnabled: totpEnabled,
		TOTPPending: totpPending,
		Passkeys:    passkeys,
	}, nil
}

func (h *handler) totpIssuer() string {
	parsed, err := url.Parse(h.ctx.Config.PublicURL)
	if err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return "HitKeep"
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

func decodeSecurityJSON(w http.ResponseWriter, r *http.Request, dest any, allowEmpty bool) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSecurityPayloadBytes))
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

func hostNameOnly(host string) string {
	if parsed, err := url.Parse("http://" + host); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	if name, _, err := net.SplitHostPort(host); err == nil && name != "" {
		return name
	}
	return host
}
