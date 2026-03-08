package user

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/entitlements"
	"hitkeep/internal/security"
	serverauth "hitkeep/internal/server/auth"
	"hitkeep/internal/server/shared"
)

func setupUserSecurityTestEnv(t *testing.T) (*handler, *database.Store, uuid.UUID) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	hashed, err := serverauth.HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	conf := &config.Config{
		PublicURL: "http://localhost:8080",
		JWTSecret: "test-secret",
	}

	ctx := &shared.Context{
		Store:        store,
		TenantStores: database.NewTenantStoreManager(store, t.TempDir()),
		Config:       conf,
		Entitlements: entitlements.NewDefaultProvider(),
	}

	return &handler{ctx: ctx}, store, userID
}

func withTestUser(req *http.Request, userID uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
}

func TestTOTPSetupLifecycle(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	startReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/security/totp/setup/start", bytes.NewReader([]byte("{}"))), userID)
	startW := httptest.NewRecorder()
	h.handleStartTOTPSetup().ServeHTTP(startW, startReq)
	if startW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, startW.Code)
	}

	var setup api.UserTOTPSetup
	if err := json.NewDecoder(startW.Body).Decode(&setup); err != nil {
		t.Fatalf("failed to decode totp setup response: %v", err)
	}
	if setup.Secret == "" {
		t.Fatalf("expected non-empty secret")
	}
	if setup.OTPAuthURL == "" {
		t.Fatalf("expected non-empty otpauth url")
	}

	code, err := security.GenerateCurrentTOTPCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate test totp code: %v", err)
	}
	verifyBody, _ := json.Marshal(map[string]string{"code": code})
	verifyReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/security/totp/setup/verify", bytes.NewReader(verifyBody)), userID)
	verifyW := httptest.NewRecorder()
	h.handleVerifyTOTPSetup().ServeHTTP(verifyW, verifyReq)
	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, verifyW.Code)
	}

	var status api.UserSecurityStatus
	if err := json.NewDecoder(verifyW.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode security status: %v", err)
	}
	if !status.TOTPEnabled {
		t.Fatalf("expected totp to be enabled")
	}

	disableCode, err := security.GenerateCurrentTOTPCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate disable totp code: %v", err)
	}
	disableBody, _ := json.Marshal(map[string]string{"code": disableCode})
	disableReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/security/totp/disable", bytes.NewReader(disableBody)), userID)
	disableW := httptest.NewRecorder()
	h.handleDisableTOTP().ServeHTTP(disableW, disableReq)
	if disableW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, disableW.Code)
	}

	statusReq := withTestUser(httptest.NewRequest(http.MethodGet, "/api/user/security", nil), userID)
	statusW := httptest.NewRecorder()
	h.handleGetUserSecurityStatus().ServeHTTP(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusW.Code)
	}
	if err := json.NewDecoder(statusW.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode security status response: %v", err)
	}
	if status.TOTPEnabled {
		t.Fatalf("expected totp to be disabled")
	}
}

func TestPasskeyRegistrationLifecycle(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	startBody, _ := json.Marshal(map[string]string{"name": "My Laptop"})
	startReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/security/passkeys/register/start", bytes.NewReader(startBody)), userID)
	startW := httptest.NewRecorder()
	h.handleStartPasskeyRegistration().ServeHTTP(startW, startReq)
	if startW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, startW.Code)
	}

	var begin passkeyRegistrationStartResponse
	if err := json.NewDecoder(startW.Body).Decode(&begin); err != nil {
		t.Fatalf("failed to decode passkey registration start response: %v", err)
	}
	if begin.PublicKey.Challenge == "" {
		t.Fatalf("expected non-empty challenge")
	}
	if begin.PublicKey.AuthenticatorSelection.UserVerification != "required" {
		t.Fatalf("expected required user verification, got %q", begin.PublicKey.AuthenticatorSelection.UserVerification)
	}

	clientData, _ := json.Marshal(map[string]string{
		"type":      "webauthn.create",
		"challenge": begin.PublicKey.Challenge,
		"origin":    "http://localhost:8080",
	})
	clientDataB64 := base64.RawURLEncoding.EncodeToString(clientData)
	publicKeyB64 := mustGeneratePublicKeyB64(t)

	finishBody, _ := json.Marshal(map[string]any{
		"credential_id":    "cred-test-1",
		"client_data_json": clientDataB64,
		"public_key":       publicKeyB64,
		"transports":       []string{"internal"},
	})
	finishReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/security/passkeys/register/finish", bytes.NewReader(finishBody)), userID)
	finishW := httptest.NewRecorder()
	h.handleFinishPasskeyRegistration().ServeHTTP(finishW, finishReq)
	if finishW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, finishW.Code)
	}

	var status api.UserSecurityStatus
	if err := json.NewDecoder(finishW.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode security status after passkey finish: %v", err)
	}
	if len(status.Passkeys) != 1 {
		t.Fatalf("expected 1 passkey, got %d", len(status.Passkeys))
	}
	if status.Passkeys[0].Name != "My Laptop" {
		t.Fatalf("expected stored passkey name to use requested registration name")
	}

	deleteReq := withTestUser(httptest.NewRequest(http.MethodDelete, "/api/user/security/passkeys/"+status.Passkeys[0].ID.String(), nil), userID)
	deleteReq.SetPathValue("id", status.Passkeys[0].ID.String())
	deleteW := httptest.NewRecorder()
	h.handleDeleteUserPasskey().ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, deleteW.Code)
	}

	statusReq := withTestUser(httptest.NewRequest(http.MethodGet, "/api/user/security", nil), userID)
	statusW := httptest.NewRecorder()
	h.handleGetUserSecurityStatus().ServeHTTP(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusW.Code)
	}
	if err := json.NewDecoder(statusW.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode final security status response: %v", err)
	}
	if len(status.Passkeys) != 0 {
		t.Fatalf("expected 0 passkeys after delete, got %d", len(status.Passkeys))
	}
}

func mustGeneratePublicKeyB64(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test ecdsa key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal test public key: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(der)
}
