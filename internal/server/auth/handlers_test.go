package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/security"
	"hitkeep/internal/server/shared"
)

func setupAuthTestEnv(t *testing.T) (*handler, *database.Store) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	conf := &config.Config{
		PublicURL: "http://localhost:8080",
		JWTSecret: "test-secret",
	}

	ctx := &shared.Context{
		Store:  store,
		Config: conf,
	}

	return &handler{ctx: ctx}, store
}

func TestHandleCreateInitialUser(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	body, err := json.Marshal(map[string]string{
		"email":      "admin@example.com",
		"password":   "password123",
		"given_name": "Ada",
		"last_name":  "Lovelace",
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/initial-user", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handle := h.handleCreateInitialUser()
	handle.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["token"] == "" {
		t.Fatalf("expected token in response")
	}

	count, err := store.GetUserCount(context.Background())
	if err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}

	user, err := store.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("failed to fetch created user: %v", err)
	}
	if user == nil {
		t.Fatalf("expected created user to exist")
	}
	if user.GivenName != "Ada" || user.LastName != "Lovelace" {
		t.Fatalf("expected given/last name to be persisted, got %+v", user)
	}

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("failed to get default tenant: %v", err)
	}
	defaultTenant, err := store.GetTenant(context.Background(), defaultTenantID)
	if err != nil {
		t.Fatalf("failed to fetch default tenant: %v", err)
	}
	if defaultTenant == nil {
		t.Fatalf("expected default tenant to exist")
	}
	if defaultTenant.Name != "Ada's Team" {
		t.Fatalf("expected default tenant name %q, got %q", "Ada's Team", defaultTenant.Name)
	}

	// Second call should be blocked once setup is complete.
	req2 := httptest.NewRequest(http.MethodPost, "/api/initial-user", bytes.NewReader(body))
	w2 := httptest.NewRecorder()
	handle.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w2.Code)
	}
}

func TestHandleLogin(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if userID == uuid.Nil {
		t.Fatalf("expected valid user ID")
	}

	t.Run("invalid credentials", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"email":    "user@example.com",
			"password": "wrongpass",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleLogin().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"email":    "user@example.com",
			"password": "password123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleLogin().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		cookies := w.Header().Values("Set-Cookie")
		found := false
		for _, cookie := range cookies {
			if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected auth cookie to be set")
		}
	})
}

func TestHandleChangePassword(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	t.Run("wrong current password", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": "wrongpass",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/user/password", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
		w := httptest.NewRecorder()
		h.handleChangePassword().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": "password123",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/user/password", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
		w := httptest.NewRecorder()
		h.handleChangePassword().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		user, err := store.GetUserByID(context.Background(), userID)
		if err != nil {
			t.Fatalf("failed to load user: %v", err)
		}
		if user == nil {
			t.Fatalf("expected user")
		}
		match, err := verifyPassword("newpassword123", user.Password)
		if err != nil {
			t.Fatalf("failed to verify password: %v", err)
		}
		if !match {
			t.Fatalf("expected password to be updated")
		}
	})
}

func TestHandleLogout(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	token, err := store.CreateRememberMeToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to create remember me token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.RememberMeCookieName, Value: token})
	w := httptest.NewRecorder()
	h.handleLogout().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Token should be deleted after logout.
	validatedUser, err := store.ValidateRememberMeToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to validate remember me token: %v", err)
	}
	if validatedUser != uuid.Nil {
		t.Fatalf("expected remember me token to be deleted")
	}
}

func TestHandlePasskeyLogin(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test ecdsa key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal test public key: %v", err)
	}
	publicKeyB64 := base64.RawURLEncoding.EncodeToString(publicKeyDER)

	_, err = store.CreateUserPasskey(context.Background(), userID, "Test Passkey", "cred-login-1", publicKeyB64, nil)
	if err != nil {
		t.Fatalf("failed to create user passkey: %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/start", bytes.NewReader([]byte("{}")))
	startW := httptest.NewRecorder()
	h.handlePasskeyLoginStart().ServeHTTP(startW, startReq)
	if startW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, startW.Code)
	}

	var startResp passkeyLoginStartResponse
	if err := json.NewDecoder(startW.Body).Decode(&startResp); err != nil {
		t.Fatalf("failed to decode passkey start response: %v", err)
	}
	if startResp.ChallengeToken == "" || startResp.PublicKey.Challenge == "" {
		t.Fatalf("expected challenge token and challenge")
	}
	if startResp.PublicKey.UserVerification != "required" {
		t.Fatalf("expected required user verification, got %q", startResp.PublicKey.UserVerification)
	}

	clientDataJSON, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": startResp.PublicKey.Challenge,
		"origin":    "http://localhost:8080",
	})
	clientDataHash := sha256.Sum256(clientDataJSON)

	rpIDHash := sha256.Sum256([]byte("localhost"))
	authData := make([]byte, 37)
	copy(authData[:32], rpIDHash[:])
	authData[32] = 0x05 // User present + user verified
	binary.BigEndian.PutUint32(authData[33:37], 1)

	signedPayload := make([]byte, 0, len(authData)+len(clientDataHash))
	signedPayload = append(signedPayload, authData...)
	signedPayload = append(signedPayload, clientDataHash[:]...)
	digest := sha256.Sum256(signedPayload)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("failed to create assertion signature: %v", err)
	}

	finishBody, _ := json.Marshal(map[string]any{
		"challenge_token":    startResp.ChallengeToken,
		"credential_id":      "cred-login-1",
		"client_data_json":   base64.RawURLEncoding.EncodeToString(clientDataJSON),
		"authenticator_data": base64.RawURLEncoding.EncodeToString(authData),
		"signature":          base64.RawURLEncoding.EncodeToString(signature),
		"remember_me":        true,
	})
	finishReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/finish", bytes.NewReader(finishBody))
	finishW := httptest.NewRecorder()
	h.handlePasskeyLoginFinish().ServeHTTP(finishW, finishReq)
	if finishW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, finishW.Code)
	}

	cookies := finishW.Header().Values("Set-Cookie")
	foundAuth := false
	foundRemember := false
	for _, cookie := range cookies {
		if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
			foundAuth = true
		}
		if bytes.Contains([]byte(cookie), []byte(auth.RememberMeCookieName+"=")) {
			foundRemember = true
		}
	}
	if !foundAuth {
		t.Fatalf("expected auth cookie to be set on passkey login")
	}
	if !foundRemember {
		t.Fatalf("expected remember me cookie to be set on passkey login")
	}
}

func TestHandlePasskeyLoginRejectsMissingUserVerification(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user-uv@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test ecdsa key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal test public key: %v", err)
	}
	publicKeyB64 := base64.RawURLEncoding.EncodeToString(publicKeyDER)

	_, err = store.CreateUserPasskey(context.Background(), userID, "Test Passkey", "cred-login-uv", publicKeyB64, nil)
	if err != nil {
		t.Fatalf("failed to create user passkey: %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/start", bytes.NewReader([]byte("{}")))
	startW := httptest.NewRecorder()
	h.handlePasskeyLoginStart().ServeHTTP(startW, startReq)
	if startW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, startW.Code)
	}

	var startResp passkeyLoginStartResponse
	if err := json.NewDecoder(startW.Body).Decode(&startResp); err != nil {
		t.Fatalf("failed to decode passkey start response: %v", err)
	}

	clientDataJSON, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": startResp.PublicKey.Challenge,
		"origin":    "http://localhost:8080",
	})
	clientDataHash := sha256.Sum256(clientDataJSON)

	rpIDHash := sha256.Sum256([]byte("localhost"))
	authData := make([]byte, 37)
	copy(authData[:32], rpIDHash[:])
	authData[32] = 0x01 // User present only; missing UV bit.
	binary.BigEndian.PutUint32(authData[33:37], 1)

	signedPayload := make([]byte, 0, len(authData)+len(clientDataHash))
	signedPayload = append(signedPayload, authData...)
	signedPayload = append(signedPayload, clientDataHash[:]...)
	digest := sha256.Sum256(signedPayload)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("failed to create assertion signature: %v", err)
	}

	finishBody, _ := json.Marshal(map[string]any{
		"challenge_token":    startResp.ChallengeToken,
		"credential_id":      "cred-login-uv",
		"client_data_json":   base64.RawURLEncoding.EncodeToString(clientDataJSON),
		"authenticator_data": base64.RawURLEncoding.EncodeToString(authData),
		"signature":          base64.RawURLEncoding.EncodeToString(signature),
		"remember_me":        false,
	})
	finishReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/finish", bytes.NewReader(finishBody))
	finishW := httptest.NewRecorder()
	h.handlePasskeyLoginFinish().ServeHTTP(finishW, finishReq)
	if finishW.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, finishW.Code)
	}
}

func TestHandleLoginMFARequiredWithTOTPOnly(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "mfa-user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	totpSecret := "JBSWY3DPEHPK3PXP"
	if err := store.EnableUserTOTP(context.Background(), userID, totpSecret); err != nil {
		t.Fatalf("failed to enable user totp: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"email":    "mfa-user@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp loginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if resp.Status != "mfa_required" {
		t.Fatalf("expected mfa_required status, got %q", resp.Status)
	}
	if resp.ChallengeToken == "" {
		t.Fatalf("expected challenge token when mfa is required")
	}
	if len(resp.Factors) != 1 || resp.Factors[0] != "totp" {
		t.Fatalf("expected only totp factor, got %v", resp.Factors)
	}
	if resp.Passkey != nil {
		t.Fatalf("expected no passkey options for totp-only user")
	}

	for _, cookie := range w.Header().Values("Set-Cookie") {
		if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
			t.Fatalf("did not expect auth cookie before mfa completion")
		}
	}
}

func TestHandleLoginMFARequiredWithPasskeyOnly(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "mfa-passkey-only@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test ecdsa key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal test public key: %v", err)
	}
	publicKeyB64 := base64.RawURLEncoding.EncodeToString(publicKeyDER)
	_, err = store.CreateUserPasskey(context.Background(), userID, "MFA Passkey", "cred-mfa-passkey-only-1", publicKeyB64, nil)
	if err != nil {
		t.Fatalf("failed to create user passkey: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"email":    "mfa-passkey-only@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp loginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if resp.Status != "mfa_required" {
		t.Fatalf("expected mfa_required status, got %q", resp.Status)
	}
	if resp.ChallengeToken == "" {
		t.Fatalf("expected challenge token when mfa is required")
	}
	if len(resp.Factors) != 1 || resp.Factors[0] != "passkey" {
		t.Fatalf("expected only passkey factor, got %v", resp.Factors)
	}
	if resp.Passkey == nil || resp.Passkey.Challenge == "" {
		t.Fatalf("expected passkey request options in mfa response")
	}

	for _, cookie := range w.Header().Values("Set-Cookie") {
		if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
			t.Fatalf("did not expect auth cookie before mfa completion")
		}
	}
}

func TestHandleMFATOTPVerify(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "mfa-verify@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	totpSecret := "JBSWY3DPEHPK3PXP"
	if err := store.EnableUserTOTP(context.Background(), userID, totpSecret); err != nil {
		t.Fatalf("failed to enable user totp: %v", err)
	}

	loginBody, _ := json.Marshal(map[string]any{
		"email":       "mfa-verify@example.com",
		"password":    "password123",
		"remember_me": true,
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginW := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, loginW.Code)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Status != "mfa_required" || loginResp.ChallengeToken == "" {
		t.Fatalf("expected mfa_required response with challenge token, got %+v", loginResp)
	}

	code, err := security.GenerateCurrentTOTPCode(totpSecret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}

	verifyBody, _ := json.Marshal(map[string]any{
		"challenge_token": loginResp.ChallengeToken,
		"code":            code,
	})
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/totp/verify", bytes.NewReader(verifyBody))
	verifyW := httptest.NewRecorder()
	h.handleMFATOTPVerify().ServeHTTP(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, verifyW.Code)
	}

	var verifyResp loginResponse
	if err := json.NewDecoder(verifyW.Body).Decode(&verifyResp); err != nil {
		t.Fatalf("failed to decode mfa verify response: %v", err)
	}
	if verifyResp.Status != "ok" {
		t.Fatalf("expected ok status after mfa verification, got %q", verifyResp.Status)
	}

	cookies := verifyW.Header().Values("Set-Cookie")
	foundAuth := false
	foundRemember := false
	for _, cookie := range cookies {
		if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
			foundAuth = true
		}
		if bytes.Contains([]byte(cookie), []byte(auth.RememberMeCookieName+"=")) {
			foundRemember = true
		}
	}
	if !foundAuth {
		t.Fatalf("expected auth cookie after mfa totp verification")
	}
	if !foundRemember {
		t.Fatalf("expected remember me cookie after mfa totp verification")
	}
}

func TestHandleMFATOTPVerifyKeepsChallengeAfterInvalidCode(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "mfa-retry@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	totpSecret := "JBSWY3DPEHPK3PXP"
	if err := store.EnableUserTOTP(context.Background(), userID, totpSecret); err != nil {
		t.Fatalf("failed to enable user totp: %v", err)
	}

	loginBody, _ := json.Marshal(map[string]any{
		"email":       "mfa-retry@example.com",
		"password":    "password123",
		"remember_me": false,
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginW := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, loginW.Code)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Status != "mfa_required" || loginResp.ChallengeToken == "" {
		t.Fatalf("expected mfa_required response with challenge token, got %+v", loginResp)
	}

	currentCode, err := security.GenerateCurrentTOTPCode(totpSecret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate baseline totp code: %v", err)
	}
	invalidCode := "0" + currentCode[1:]
	if currentCode[0] == '0' {
		invalidCode = "1" + currentCode[1:]
	}

	firstVerifyBody, _ := json.Marshal(map[string]any{
		"challenge_token": loginResp.ChallengeToken,
		"code":            invalidCode,
	})
	firstVerifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/totp/verify", bytes.NewReader(firstVerifyBody))
	firstVerifyW := httptest.NewRecorder()
	h.handleMFATOTPVerify().ServeHTTP(firstVerifyW, firstVerifyReq)

	if firstVerifyW.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d for invalid totp, got %d", http.StatusUnauthorized, firstVerifyW.Code)
	}

	validCode, err := security.GenerateCurrentTOTPCode(totpSecret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}
	secondVerifyBody, _ := json.Marshal(map[string]any{
		"challenge_token": loginResp.ChallengeToken,
		"code":            validCode,
	})
	secondVerifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/totp/verify", bytes.NewReader(secondVerifyBody))
	secondVerifyW := httptest.NewRecorder()
	h.handleMFATOTPVerify().ServeHTTP(secondVerifyW, secondVerifyReq)

	if secondVerifyW.Code != http.StatusOK {
		t.Fatalf("expected status %d after retry with valid code, got %d", http.StatusOK, secondVerifyW.Code)
	}
}

func TestHandleMFAPasskeyLoginFinish(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "mfa-passkey@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	totpSecret := "JBSWY3DPEHPK3PXP"
	if err := store.EnableUserTOTP(context.Background(), userID, totpSecret); err != nil {
		t.Fatalf("failed to enable user totp: %v", err)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test ecdsa key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal test public key: %v", err)
	}
	publicKeyB64 := base64.RawURLEncoding.EncodeToString(publicKeyDER)

	_, err = store.CreateUserPasskey(context.Background(), userID, "MFA Passkey", "cred-mfa-1", publicKeyB64, nil)
	if err != nil {
		t.Fatalf("failed to create user passkey: %v", err)
	}

	loginBody, _ := json.Marshal(map[string]any{
		"email":       "mfa-passkey@example.com",
		"password":    "password123",
		"remember_me": true,
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginW := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, loginW.Code)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Status != "mfa_required" || loginResp.ChallengeToken == "" {
		t.Fatalf("expected mfa_required response with challenge token, got %+v", loginResp)
	}
	if !containsFactor(loginResp.Factors, "totp") || !containsFactor(loginResp.Factors, "passkey") {
		t.Fatalf("expected both totp and passkey factors, got %v", loginResp.Factors)
	}
	if loginResp.Passkey == nil || loginResp.Passkey.Challenge == "" {
		t.Fatalf("expected passkey request options in mfa response")
	}
	if loginResp.Passkey.UserVerification != "required" {
		t.Fatalf("expected required user verification for mfa passkey flow, got %q", loginResp.Passkey.UserVerification)
	}

	clientDataJSON, _ := json.Marshal(map[string]string{
		"type":      "webauthn.get",
		"challenge": loginResp.Passkey.Challenge,
		"origin":    "http://localhost:8080",
	})
	clientDataHash := sha256.Sum256(clientDataJSON)

	rpIDHash := sha256.Sum256([]byte("localhost"))
	authData := make([]byte, 37)
	copy(authData[:32], rpIDHash[:])
	authData[32] = 0x05 // User present + user verified
	binary.BigEndian.PutUint32(authData[33:37], 1)

	signedPayload := make([]byte, 0, len(authData)+len(clientDataHash))
	signedPayload = append(signedPayload, authData...)
	signedPayload = append(signedPayload, clientDataHash[:]...)
	digest := sha256.Sum256(signedPayload)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("failed to create assertion signature: %v", err)
	}

	finishBody, _ := json.Marshal(map[string]any{
		"challenge_token":    loginResp.ChallengeToken,
		"credential_id":      "cred-mfa-1",
		"client_data_json":   base64.RawURLEncoding.EncodeToString(clientDataJSON),
		"authenticator_data": base64.RawURLEncoding.EncodeToString(authData),
		"signature":          base64.RawURLEncoding.EncodeToString(signature),
		// This should be ignored for MFA flow; remember-me comes from the challenge created on /api/login.
		"remember_me": false,
	})
	finishReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/finish", bytes.NewReader(finishBody))
	finishW := httptest.NewRecorder()
	h.handlePasskeyLoginFinish().ServeHTTP(finishW, finishReq)
	if finishW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, finishW.Code)
	}

	cookies := finishW.Header().Values("Set-Cookie")
	foundAuth := false
	foundRemember := false
	for _, cookie := range cookies {
		if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
			foundAuth = true
		}
		if bytes.Contains([]byte(cookie), []byte(auth.RememberMeCookieName+"=")) {
			foundRemember = true
		}
	}
	if !foundAuth {
		t.Fatalf("expected auth cookie after mfa passkey verification")
	}
	if !foundRemember {
		t.Fatalf("expected remember me cookie to follow login challenge in mfa flow")
	}
}

func TestHandleAcceptInviteActivatesPendingTeamInvite(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	ownerHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash owner password: %v", err)
	}
	ownerID, err := store.CreateUser(context.Background(), "owner-invite@example.com", ownerHash)
	if err != nil {
		t.Fatalf("failed to create owner user: %v", err)
	}

	inviteeHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash invitee password: %v", err)
	}
	inviteeID, err := store.CreateUser(context.Background(), "accept-invite@example.com", inviteeHash)
	if err != nil {
		t.Fatalf("failed to create invitee user: %v", err)
	}

	teamID := uuid.New()
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		teamID, "Accept Invite", time.Now().UTC(),
	); err != nil {
		t.Fatalf("failed to insert team: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), teamID, ownerID, database.TenantRoleOwner, ownerID); err != nil {
		t.Fatalf("failed to add owner to team: %v", err)
	}
	if _, err := store.CreateTeamInvite(context.Background(), teamID, "accept-invite@example.com", database.TenantRoleAdmin, &inviteeID, ownerID); err != nil {
		t.Fatalf("failed to create team invite: %v", err)
	}

	token, err := store.CreatePasswordResetToken(context.Background(), "accept-invite@example.com")
	if err != nil {
		t.Fatalf("failed to create invite token: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"token":    token,
		"password": "new-password-123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/accept-invite", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleAcceptInvite().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	isMember, err := store.IsTenantMember(context.Background(), teamID, inviteeID)
	if err != nil {
		t.Fatalf("check invitee membership: %v", err)
	}
	if !isMember {
		t.Fatalf("expected invitee to become a team member")
	}

	role, err := store.GetTenantRole(context.Background(), teamID, inviteeID)
	if err != nil {
		t.Fatalf("get invitee role: %v", err)
	}
	if role != database.TenantRoleAdmin {
		t.Fatalf("expected invitee role %q, got %q", database.TenantRoleAdmin, role)
	}

	invites, err := store.ListTeamInvites(context.Background(), teamID)
	if err != nil {
		t.Fatalf("list team invites: %v", err)
	}
	if len(invites) != 0 {
		t.Fatalf("expected no pending invites after acceptance, got %d", len(invites))
	}
}

func TestHandleLoginIncludesRecoveryCodeFactor(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "recovery-factor@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	secret, err := security.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate totp secret: %v", err)
	}
	if err := store.EnableUserTOTP(context.Background(), userID, secret); err != nil {
		t.Fatalf("enable totp: %v", err)
	}

	codes, err := security.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("generate recovery codes: %v", err)
	}
	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hash, err := security.HashRecoveryCode(code)
		if err != nil {
			t.Fatalf("hash recovery code: %v", err)
		}
		hashes = append(hashes, hash)
	}
	if err := store.ReplaceUserRecoveryCodes(context.Background(), userID, hashes); err != nil {
		t.Fatalf("replace recovery codes: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"email":    "recovery-factor@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp loginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if resp.Status != "mfa_required" {
		t.Fatalf("expected mfa_required status, got %q", resp.Status)
	}
	if !containsFactor(resp.Factors, "totp") {
		t.Fatalf("expected totp factor, got %v", resp.Factors)
	}
	if !containsFactor(resp.Factors, "recovery_code") {
		t.Fatalf("expected recovery code factor, got %v", resp.Factors)
	}
}

func TestHandleMFARecoveryCodeVerify(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "recovery-verify@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	secret, err := security.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate totp secret: %v", err)
	}
	if err := store.EnableUserTOTP(context.Background(), userID, secret); err != nil {
		t.Fatalf("enable totp: %v", err)
	}

	codes, err := security.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("generate recovery codes: %v", err)
	}
	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hash, err := security.HashRecoveryCode(code)
		if err != nil {
			t.Fatalf("hash recovery code: %v", err)
		}
		hashes = append(hashes, hash)
	}
	if err := store.ReplaceUserRecoveryCodes(context.Background(), userID, hashes); err != nil {
		t.Fatalf("replace recovery codes: %v", err)
	}

	loginBody, _ := json.Marshal(map[string]any{
		"email":       "recovery-verify@example.com",
		"password":    "password123",
		"remember_me": true,
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginW := httptest.NewRecorder()
	h.handleLogin().ServeHTTP(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, loginW.Code)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResp.ChallengeToken == "" {
		t.Fatal("expected challenge token")
	}

	t.Run("invalid code", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"challenge_token": loginResp.ChallengeToken,
			"code":            "BAD-CODE",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/recovery-code/verify", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleMFARecoveryCodeVerify().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}

		challengeID, err := uuid.Parse(loginResp.ChallengeToken)
		if err != nil {
			t.Fatalf("parse challenge token: %v", err)
		}
		if _, found, err := store.GetPasskeyLoginChallenge(context.Background(), challengeID); err != nil {
			t.Fatalf("load mfa challenge: %v", err)
		} else if !found {
			t.Fatal("expected mfa challenge to remain after invalid recovery code")
		}
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"challenge_token": loginResp.ChallengeToken,
			"code":            codes[0],
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/recovery-code/verify", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleMFARecoveryCodeVerify().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		remaining, err := store.CountActiveRecoveryCodes(context.Background(), userID)
		if err != nil {
			t.Fatalf("count active recovery codes: %v", err)
		}
		if remaining != len(codes)-1 {
			t.Fatalf("expected %d remaining recovery codes, got %d", len(codes)-1, remaining)
		}

		challengeID, err := uuid.Parse(loginResp.ChallengeToken)
		if err != nil {
			t.Fatalf("parse challenge token: %v", err)
		}
		if _, found, err := store.GetPasskeyLoginChallenge(context.Background(), challengeID); err != nil {
			t.Fatalf("load mfa challenge: %v", err)
		} else if found {
			t.Fatal("expected mfa challenge to be deleted after recovery code verification")
		}

		cookies := w.Header().Values("Set-Cookie")
		foundAuth := false
		foundRemember := false
		for _, cookie := range cookies {
			if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
				foundAuth = true
			}
			if bytes.Contains([]byte(cookie), []byte(auth.RememberMeCookieName+"=")) {
				foundRemember = true
			}
		}
		if !foundAuth {
			t.Fatalf("expected auth cookie after recovery code verification")
		}
		if !foundRemember {
			t.Fatalf("expected remember me cookie after recovery code verification")
		}
	})
}

func containsFactor(factors []string, factor string) bool {
	return slices.Contains(factors, factor)
}
