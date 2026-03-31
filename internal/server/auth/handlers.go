package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"golang.org/x/text/language"

	authcore "hitkeep/internal/auth"
	"hitkeep/internal/database"
	"hitkeep/internal/mailables"
	"hitkeep/internal/mailer"
	"hitkeep/internal/security"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

func fallbackMailLocale(acceptLanguage string) string {
	tags, _, err := language.ParseAcceptLanguage(strings.TrimSpace(acceptLanguage))
	if err != nil {
		return "en"
	}

	for _, tag := range tags {
		base, _ := tag.Base()
		if base.String() == language.Und.String() || base.String() == "" {
			continue
		}
		return mailer.NormalizeLocale(base.String())
	}

	return "en"
}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("POST /api/initial-user", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleCreateInitialUser()))
	mux.HandleFunc("POST /api/login", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleLogin()))
	mux.HandleFunc("POST /api/logout", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleLogout()))
	mux.HandleFunc("POST /api/auth/forgot-password", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleForgotPassword()))
	mux.HandleFunc("POST /api/auth/reset-password", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleResetPassword()))
	mux.HandleFunc("POST /api/auth/accept-invite", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleAcceptInvite()))
	mux.HandleFunc("POST /api/auth/passkey/login/start", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handlePasskeyLoginStart()))
	mux.HandleFunc("POST /api/auth/passkey/login/finish", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handlePasskeyLoginFinish()))
	mux.HandleFunc("POST /api/auth/mfa/totp/verify", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleMFATOTPVerify()))
	mux.HandleFunc("POST /api/auth/mfa/recovery-code/verify", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleMFARecoveryCodeVerify()))
	mux.HandleFunc("POST /api/user/password", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleChangePassword()))
}

func (h *handler) handleCreateInitialUser() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
		//nolint:gosec // request payload intentionally accepts plaintext password input.
		Password  string `json:"password"`
		GivenName string `json:"given_name"`
		LastName  string `json:"last_name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}
		if h.ctx.Config.CloudHosted {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		userCount, err := h.ctx.Store.GetUserCount(r.Context())
		if err != nil {
			slog.Error("Failed to check user count during setup", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if userCount > 0 {
			http.Error(w, "Setup has already been completed.", http.StatusForbidden)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Email == "" || len(req.Password) < 8 {
			http.Error(w, "Email required; Password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		hashedPassword, err := HashPassword(req.Password)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		userID, err := h.ctx.Store.CreateUserWithNames(r.Context(), req.Email, hashedPassword, req.GivenName, req.LastName)
		if err != nil {
			slog.Error("Failed to create initial user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		token, err := authcore.GenerateToken(h.ctx.Config.JWTSecret, h.ctx.Config.PublicURL, userID)
		if err != nil {
			slog.Error("Failed to generate auth token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		isSecure := strings.HasPrefix(h.ctx.Config.PublicURL, "https://")
		authcore.SetTokenCookie(w, token, isSecure)

		//nolint:gosec // email/user_id are expected audit fields after successful account setup.
		slog.Info("Initial admin user created", "email", req.Email, "user_id", userID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"token": token,
		}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleLogin() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
		//nolint:gosec // request payload intentionally accepts plaintext password input.
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		user, err := h.ctx.Store.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			slog.Error("Database error during login", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			burnLoginKDF(req.Password)
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		match, err := verifyPassword(req.Password, user.Password)
		if err != nil {
			slog.Error("Password verification error", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !match {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		totpEnabled, err := h.ctx.Store.HasEnabledTOTP(r.Context(), user.ID)
		if err != nil {
			//nolint:gosec // user_id comes from authenticated lookup and is logged for auditability.
			slog.Error("Failed to check user totp status during login", "error", err, "user_id", user.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		passkeys, err := h.ctx.Store.ListUserPasskeys(r.Context(), user.ID)
		if err != nil {
			//nolint:gosec // user_id comes from authenticated lookup and is logged for auditability.
			slog.Error("Failed to list user passkeys during login", "error", err, "user_id", user.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		hasPasskey := len(passkeys) > 0
		recoveryCodesRemaining, err := h.ctx.Store.CountActiveRecoveryCodes(r.Context(), user.ID)
		if err != nil {
			slog.Error("Failed to count user recovery codes during login", "error", err, "user_id", user.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		hasRecoveryCode := recoveryCodesRemaining > 0

		if totpEnabled || hasPasskey {
			userID := user.ID
			var (
				challenge      string
				session        *webauthnlib.SessionData
				passkeyOptions *protocol.PublicKeyCredentialRequestOptions
			)

			if hasPasskey {
				passkeyUser, err := h.loadPasskeyUser(r.Context(), user.ID)
				if err != nil {
					slog.Error("Failed to load passkey user during login", "error", err, "user_id", user.ID)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				webAuthn, err := security.NewWebAuthn(h.ctx.Config.PublicURL, r)
				if err != nil {
					slog.Error("Failed to configure MFA passkey login", "error", err, "user_id", user.ID)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				assertion, beginSession, err := webAuthn.BeginLogin(passkeyUser)
				if err != nil {
					slog.Error("Failed to begin MFA passkey login", "error", err, "user_id", user.ID)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				challenge = beginSession.Challenge
				session = beginSession
				passkeyOptions = &assertion.Response
			} else {
				var err error
				challenge, err = security.GenerateRandomChallenge(32)
				if err != nil {
					slog.Error("Failed to generate mfa challenge for login", "error", err, "user_id", user.ID)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
			}

			expiresAt := time.Now().UTC().Add(passkeyLoginChallengeTTL)
			if session != nil && !session.Expires.IsZero() {
				expiresAt = session.Expires
			}

			if h.ctx.AuthState == nil {
				http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
				return
			}
			challengeID := h.ctx.AuthState.CreatePasskeyLoginChallenge(challenge, database.CreateLoginChallengeInput{
				UserID:     &userID,
				RememberMe: req.RememberMe,
				Flow:       "mfa",
			}, expiresAt, session)

			factors := make([]string, 0, 2)
			if totpEnabled {
				factors = append(factors, "totp")
			}
			if hasRecoveryCode {
				factors = append(factors, "recovery_code")
			}
			resp := loginResponse{
				Status:         "mfa_required",
				ChallengeToken: challengeID.String(),
				Factors:        factors,
			}

			if hasPasskey {
				resp.Factors = append(resp.Factors, "passkey")
				resp.Passkey = passkeyOptions
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				slog.Error("Failed to encode mfa-required login response", "error", err, "user_id", user.ID)
			}
			return
		}

		if err := h.issueLoginSession(r.Context(), w, user.ID, req.RememberMe); err != nil {
			slog.Error("Failed to issue login session", "error", err, "user_id", user.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		slog.Info("User logged in", "user_id", user.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(loginResponse{Status: "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

type loginResponse struct {
	Status         string                                      `json:"status"`
	ChallengeToken string                                      `json:"challenge_token,omitempty"`
	Factors        []string                                    `json:"factors,omitempty"`
	Passkey        *protocol.PublicKeyCredentialRequestOptions `json:"passkey,omitempty"`
}

func (h *handler) issueLoginSession(ctx context.Context, w http.ResponseWriter, userID uuid.UUID, rememberMe bool) error {
	token, err := authcore.GenerateToken(h.ctx.Config.JWTSecret, h.ctx.Config.PublicURL, userID)
	if err != nil {
		return fmt.Errorf("could not generate auth token: %w", err)
	}

	isSecure := strings.HasPrefix(h.ctx.Config.PublicURL, "https://")
	authcore.SetTokenCookie(w, token, isSecure)

	if rememberMe {
		rememberToken, err := h.ctx.Store.CreateRememberMeToken(ctx, userID)
		if err != nil {
			slog.Error("Failed to create remember me token", "error", err, "user_id", userID)
			return nil
		}
		authcore.SetRememberMeCookie(w, rememberToken, isSecure)
	}
	return nil
}

func HashPassword(password string) (string, error) {
	const (
		time    = 1
		memory  = 64 * 1024
		threads = 4
		keyLen  = 32
		saltLen = 16
	)
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, time, threads, b64Salt, b64Hash), nil
}

func verifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid hash format")
	}
	if parts[1] != "argon2id" {
		return false, errors.New("incompatible variant")
	}
	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return false, err
	}
	if version != argon2.Version {
		return false, errors.New("incompatible version")
	}
	var memory uint32
	var time uint32
	var threads uint8
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	// Ensure the key length doesnt exceed MaxUint32 to prevent overflow in IDKey
	if uint64(len(decodedHash)) > math.MaxUint32 {
		return false, errors.New("decoded hash too long")
	}
	//nolint:gosec // above
	keyLen := uint32(len(decodedHash))
	comparisonHash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	if subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1 {
		return true, nil
	}
	return false, nil
}

func burnLoginKDF(secret string) {
	const (
		timeCost   uint32 = 1
		memoryCost uint32 = 64 * 1024
		threads    uint8  = 4
		keyLen     uint32 = 32
	)

	// Deterministic salt keeps the operation stable and avoids external dependencies.
	salt := []byte("0123456789abcdef")
	hash := argon2.IDKey([]byte(secret), salt, timeCost, memoryCost, threads, keyLen)
	if subtle.ConstantTimeCompare(hash, hash) != 1 {
		slog.Warn("Unreachable login kdf comparison failure")
	}
}

func (h *handler) handleForgotPassword() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		user, err := h.ctx.Store.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			slog.Error("Database error checking user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// fake to prevenet enumeration
		if user == nil {
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"message": "If an account exists, a reset link has been sent."}); err != nil {
				slog.Error("Failed to encode response", "error", err)
			}
			return
		}

		token, err := h.ctx.Store.CreatePasswordResetToken(r.Context(), user.Email)
		if err != nil {
			slog.Error("Failed to create reset token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resetLink := fmt.Sprintf("%s/reset-password?token=%s", h.ctx.Config.PublicURL, token)
		locale := fallbackMailLocale(r.Header.Get("Accept-Language"))
		prefs, err := h.ctx.Store.GetUserPreferences(r.Context(), user.ID)
		if err != nil {
			slog.Warn("Failed to load user preferences for password reset", "error", err, "user_id", user.ID)
		} else if prefs != nil && strings.TrimSpace(prefs.DefaultLocale) != "" {
			locale = mailer.NormalizeLocale(prefs.DefaultLocale)
		}

		err = h.ctx.Mailer.Send(user.Email, mailables.NewPasswordReset(resetLink, locale))
		if err != nil {
			slog.Error("Failed to send password reset email", "error", err, "email", user.Email)
			// Here we actually return an error because if the mailer fails, the user is stuck.
			http.Error(w, "Failed to send email. Check server logs.", http.StatusBadGateway)
			return
		}

		slog.Info("Password reset requested", "email", user.Email)

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"message": "If an account exists, a reset link has been sent."}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleResetPassword() http.HandlerFunc {
	type request struct {
		Token string `json:"token"`
		//nolint:gosec // request payload intentionally accepts plaintext password input.
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Token == "" || len(req.Password) < 8 {
			http.Error(w, "Invalid token or password too short", http.StatusBadRequest)
			return
		}

		// 1. Hash the new password (Reusing existing logic)
		hashedPassword, err := HashPassword(req.Password)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 2. Perform the reset in the store
		err = h.ctx.Store.CompletePasswordReset(r.Context(), req.Token, hashedPassword)
		if err != nil {
			if errors.Is(err, database.ErrPasswordResetInvalid) || errors.Is(err, database.ErrPasswordResetExpired) {
				http.Error(w, "Invalid or expired link", http.StatusBadRequest)
				return
			}

			slog.Error("Failed to complete password reset", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		slog.Info("Password reset successful", "token_mask", req.Token[:4]+"...")

		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Password updated successfully"})
		if err != nil {
			slog.Error("Failed to complete password reset", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}
func (h *handler) handleAcceptInvite() http.HandlerFunc {
	type request struct {
		Token string `json:"token"`
		//nolint:gosec // request payload intentionally accepts plaintext password input.
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Token == "" || len(req.Password) < 8 {
			http.Error(w, "Invalid token or password too short", http.StatusBadRequest)
			return
		}

		email, err := h.ctx.Store.ResolvePasswordResetEmail(r.Context(), req.Token)
		if err != nil {
			if errors.Is(err, database.ErrPasswordResetInvalid) || errors.Is(err, database.ErrPasswordResetExpired) {
				http.Error(w, "Invalid or expired link", http.StatusBadRequest)
				return
			}
			slog.Error("Failed to resolve invite token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 1. Hash the new password
		hashedPassword, err := HashPassword(req.Password)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 2. Perform the reset in the store.
		err = h.ctx.Store.CompletePasswordReset(r.Context(), req.Token, hashedPassword)
		if err != nil {
			if errors.Is(err, database.ErrPasswordResetInvalid) || errors.Is(err, database.ErrPasswordResetExpired) {
				http.Error(w, "Invalid or expired link", http.StatusBadRequest)
				return
			}

			slog.Error("Failed to complete invite acceptance", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		user, err := h.ctx.Store.GetUserByEmail(r.Context(), email)
		if err != nil {
			slog.Error("Failed to load invited user after password reset", "error", err, "email", email)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "Invalid or expired link", http.StatusBadRequest)
			return
		}

		if h.ctx.Config.CloudHosted {
			pendingInvites, err := h.ctx.Store.ListPendingTeamInvitesByEmail(r.Context(), email)
			if err != nil {
				slog.Error("Failed to list pending cloud invites", "error", err, "email", email, "user_id", user.ID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			targetTeams := make(map[uuid.UUID]struct{})
			for _, invite := range pendingInvites {
				isMember, err := h.ctx.Store.IsTenantMember(r.Context(), invite.TeamID, user.ID)
				if err != nil {
					slog.Error("Failed to check cloud invite membership", "error", err, "email", email, "user_id", user.ID, "team_id", invite.TeamID)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				if !isMember {
					targetTeams[invite.TeamID] = struct{}{}
				}
			}

			teamCount, err := h.ctx.Store.CountUserNonDefaultTeams(r.Context(), user.ID)
			if err != nil {
				slog.Error("Failed to count cloud invite teams", "error", err, "email", email, "user_id", user.ID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if len(targetTeams) > 1 || (teamCount > 0 && len(targetTeams) > 0) {
				http.Error(w, "Managed cloud accounts are limited to one team", http.StatusForbidden)
				return
			}
		}

		acceptedInvites, err := h.ctx.Store.AcceptTeamInvitesByEmail(r.Context(), email, user.ID)
		if err != nil {
			slog.Error("Failed to accept team invites", "error", err, "email", email, "user_id", user.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		for _, invite := range acceptedInvites {
			if err := h.ctx.Store.AppendTeamAuditEntry(r.Context(), invite.TeamID, user.ID, "member.invite_accepted", fmt.Sprintf("Invitation accepted by %s", email), &user.ID); err != nil {
				slog.Warn("Failed to append invite acceptance audit entry", "error", err, "team_id", invite.TeamID, "user_id", user.ID)
			}
		}

		slog.Info("Invite accepted", "token_mask", req.Token[:4]+"...")

		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Account set up successfully. Please log in."})
		if err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleChangePassword() http.HandlerFunc {
	type request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if h.ctx.Store == nil {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if len(req.NewPassword) < 8 {
			http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		user, err := h.ctx.Store.GetUserByID(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to fetch user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Verify old password
		match, err := verifyPassword(req.CurrentPassword, user.Password)
		if err != nil {
			slog.Error("Error verifying password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !match {
			http.Error(w, "Current password is incorrect", http.StatusForbidden)
			return
		}

		// Hash new password
		newHash, err := HashPassword(req.NewPassword)
		if err != nil {
			slog.Error("Failed to hash new password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := h.ctx.Store.UpdatePasswordByID(r.Context(), userID.String(), newHash); err != nil {
			slog.Error("Failed to update password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		slog.Info("User changed password", "user_id", userID)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (h *handler) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isSecure := strings.HasPrefix(h.ctx.Config.PublicURL, "https://")

		// If we have a remember me cookie, delete the token from DB
		if cookie, err := r.Cookie(authcore.RememberMeCookieName); err == nil {
			if err := h.ctx.Store.DeleteRememberMeToken(r.Context(), cookie.Value); err != nil {
				slog.Error("Failed to delete remember me token", "error", err)
			}
		}

		authcore.ClearCookies(w, isSecure)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
