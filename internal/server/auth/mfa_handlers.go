package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
	"hitkeep/internal/security"
)

type mfaTotpVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
}

func (h *handler) loadMFAChallenge(w http.ResponseWriter, r *http.Request, challengeToken string) (database.LoginChallenge, bool) {
	challengeID, err := uuid.Parse(strings.TrimSpace(challengeToken))
	if err != nil {
		http.Error(w, "Invalid challenge token", http.StatusBadRequest)
		return database.LoginChallenge{}, false
	}

	challenge, found, err := h.ctx.Store.GetPasskeyLoginChallenge(r.Context(), challengeID)
	if err != nil {
		slog.Error("Failed to load mfa challenge", "error", err, "challenge_id", challengeID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return database.LoginChallenge{}, false
	}
	if !found {
		http.Error(w, "MFA challenge not found", http.StatusConflict)
		return database.LoginChallenge{}, false
	}
	if strings.TrimSpace(challenge.Flow) != "mfa" || !challenge.HasUserID {
		http.Error(w, "Invalid MFA challenge", http.StatusForbidden)
		return database.LoginChallenge{}, false
	}

	if time.Now().UTC().After(challenge.ExpiresAt.UTC()) {
		if err := h.ctx.Store.DeletePasskeyLoginChallenge(r.Context(), challengeID); err != nil {
			slog.Warn("Failed to delete expired mfa challenge", "error", err, "challenge_id", challengeID)
		}
		http.Error(w, "MFA challenge expired", http.StatusGone)
		return database.LoginChallenge{}, false
	}

	return challenge, true
}

func (h *handler) handleMFATOTPVerify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req mfaTotpVerifyRequest
		if !decodeAuthJSON(w, r, &req, false) {
			return
		}

		challenge, ok := h.loadMFAChallenge(w, r, req.ChallengeToken)
		if !ok {
			return
		}

		secret, enabled, err := h.ctx.Store.GetUserTOTPSecret(r.Context(), challenge.UserID)
		if err != nil {
			slog.Error("Failed to load user totp secret for mfa verify", "error", err, "user_id", challenge.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !enabled {
			http.Error(w, "TOTP is not enabled", http.StatusConflict)
			return
		}
		if !security.ValidateTOTPCodeStrict(secret, req.Code, time.Now().UTC()) {
			http.Error(w, "Invalid TOTP code", http.StatusUnauthorized)
			return
		}

		challengeID, err := uuid.Parse(strings.TrimSpace(req.ChallengeToken))
		if err != nil {
			http.Error(w, "Invalid challenge token", http.StatusBadRequest)
			return
		}
		if err := h.ctx.Store.DeletePasskeyLoginChallenge(r.Context(), challengeID); err != nil {
			slog.Error("Failed to consume mfa challenge after totp verification", "error", err, "challenge_id", challengeID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := h.issueLoginSession(r.Context(), w, challenge.UserID, challenge.RememberMe); err != nil {
			slog.Error("Failed to issue login session after mfa totp verification", "error", err, "user_id", challenge.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(loginResponse{Status: "ok"}); err != nil {
			slog.Error("Failed to encode mfa totp verification response", "error", err, "user_id", challenge.UserID)
		}
	}
}

func (h *handler) handleMFARecoveryCodeVerify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req mfaTotpVerifyRequest
		if !decodeAuthJSON(w, r, &req, false) {
			return
		}

		challenge, ok := h.loadMFAChallenge(w, r, req.ChallengeToken)
		if !ok {
			return
		}

		_, consumed, err := h.ctx.Store.ConsumeRecoveryCode(r.Context(), challenge.UserID, req.Code)
		if err != nil {
			slog.Error("Failed to consume recovery code for mfa verify", "error", err, "user_id", challenge.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !consumed {
			http.Error(w, "Invalid recovery code", http.StatusUnauthorized)
			return
		}

		challengeID, err := uuid.Parse(strings.TrimSpace(req.ChallengeToken))
		if err != nil {
			http.Error(w, "Invalid challenge token", http.StatusBadRequest)
			return
		}
		if err := h.ctx.Store.DeletePasskeyLoginChallenge(r.Context(), challengeID); err != nil {
			slog.Error("Failed to consume mfa challenge after recovery code verification", "error", err, "challenge_id", challengeID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := h.issueLoginSession(r.Context(), w, challenge.UserID, challenge.RememberMe); err != nil {
			slog.Error("Failed to issue login session after recovery code verification", "error", err, "user_id", challenge.UserID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(loginResponse{Status: "ok"}); err != nil {
			slog.Error("Failed to encode recovery code verification response", "error", err, "user_id", challenge.UserID)
		}
	}
}
