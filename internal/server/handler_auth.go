package server

import (
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

	"golang.org/x/crypto/argon2"

	"hitkeep/internal/auth"
	"hitkeep/internal/mailables"
)

func (s *Server) handleCreateInitialUser() http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		userCount, err := s.store.GetUserCount(r.Context())
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

		hashedPassword, err := hashPassword(req.Password)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		userID, err := s.store.CreateUser(r.Context(), req.Email, hashedPassword)
		if err != nil {
			slog.Error("Failed to create initial user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		token, err := auth.GenerateToken(s.conf.JWTSecret, s.conf.PublicURL, userID)
		if err != nil {
			slog.Error("Failed to generate auth token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		isSecure := strings.HasPrefix(s.conf.PublicURL, "https://")
		auth.SetTokenCookie(w, token, isSecure)

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

func (s *Server) handleLogin() http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		user, err := s.store.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			slog.Error("Database error during login", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
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

		token, err := auth.GenerateToken(s.conf.JWTSecret, s.conf.PublicURL, user.ID)
		if err != nil {
			slog.Error("Failed to generate auth token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		isSecure := strings.HasPrefix(s.conf.PublicURL, "https://")
		auth.SetTokenCookie(w, token, isSecure)

		slog.Info("User logged in", "user_id", user.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func hashPassword(password string) (string, error) {
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

func (s *Server) handleForgotPassword() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		user, err := s.store.GetUserByEmail(r.Context(), req.Email)
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

		token, err := s.store.CreatePasswordResetToken(r.Context(), user.Email)
		if err != nil {
			slog.Error("Failed to create reset token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.conf.PublicURL, token)

		err = s.mailer.Send(user.Email, mailables.NewPasswordReset(resetLink))
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

func (s *Server) handleResetPassword() http.HandlerFunc {
	type request struct {
		Token    string `json:"token"`
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
		hashedPassword, err := hashPassword(req.Password)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 2. Perform the reset in the store
		err = s.store.CompletePasswordReset(r.Context(), req.Token, hashedPassword)
		if err != nil {
			// Don't leak exact DB errors to client, but "invalid token" is safe enough
			if err.Error() == "invalid or expired token" || err.Error() == "token expired" {
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

func (s *Server) handleChangePassword() http.HandlerFunc {
	type request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if s.store == nil {
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

		user, err := s.store.GetUserByID(r.Context(), userID)
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
		newHash, err := hashPassword(req.NewPassword)
		if err != nil {
			slog.Error("Failed to hash new password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := s.store.UpdatePasswordByID(r.Context(), userID.String(), newHash); err != nil {
			slog.Error("Failed to update password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		slog.Info("User changed password", "user_id", userID)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
