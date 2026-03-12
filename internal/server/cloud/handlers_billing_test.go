//go:build billing

package cloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"
	"golang.org/x/time/rate"

	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

type fakeStripeClient struct {
	lastCustomerInput *createCustomerInput
	lastCheckoutInput *createCheckoutSessionInput
	lastPortalInput   *createPortalSessionInput
	lastChargeID      string
	chargeCustomerID  string
}

func (f *fakeStripeClient) CreateCustomer(_ context.Context, input createCustomerInput) (string, error) {
	f.lastCustomerInput = &input
	return "cus_test", nil
}

func (f *fakeStripeClient) CreateCheckoutSession(_ context.Context, input createCheckoutSessionInput) (*checkoutSessionOutput, error) {
	f.lastCheckoutInput = &input
	return &checkoutSessionOutput{
		ID:  "cs_test",
		URL: "https://checkout.stripe.test/session",
	}, nil
}

func (f *fakeStripeClient) CreatePortalSession(_ context.Context, input createPortalSessionInput) (*portalSessionOutput, error) {
	f.lastPortalInput = &input
	return &portalSessionOutput{
		ID:  "bps_test",
		URL: "https://billing.stripe.test/session",
	}, nil
}

func (f *fakeStripeClient) GetCharge(_ context.Context, chargeID string) (*stripeChargeOutput, error) {
	f.lastChargeID = chargeID
	customerID := f.chargeCustomerID
	if customerID == "" {
		customerID = "cus_dispute"
	}
	return &stripeChargeOutput{
		ID:         chargeID,
		CustomerID: customerID,
	}, nil
}

type fakeWebhookVerifier struct {
	event stripe.Event
	err   error
}

func (f fakeWebhookVerifier) ConstructEvent(_ []byte, _ string, _ string) (stripe.Event, error) {
	return f.event, f.err
}

func setupCloudTestHandler(t *testing.T) (*handler, *database.Store) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	h := &handler{
		ctx: &shared.Context{
			Store: store,
			Config: &config.Config{
				PublicURL:                   "https://cloud.hitkeep.eu",
				JWTSecret:                   "test-secret",
				CloudHosted:                 true,
				CloudSignupEnabled:          true,
				CloudJurisdiction:           "EU",
				StripeSecretKey:             "sk_test_123",
				StripePortalConfigurationID: "bpc_test_123",
				StripeWebhookSecret:         "whsec_test_123",
				StripePriceProMonthly:       "price_pro",
				StripePriceBusinessMonthly:  "price_business",
			},
		},
		stripe:   &fakeStripeClient{},
		webhooks: fakeWebhookVerifier{},
	}

	return h, store
}

func setupSignedCloudWebhookTestHandler(t *testing.T) (*handler, *database.Store, *fakeStripeClient) {
	t.Helper()

	h, store := setupCloudTestHandler(t)
	stripeClient, ok := h.stripe.(*fakeStripeClient)
	if !ok {
		t.Fatal("expected fake stripe client")
	}
	h.webhooks = stripeWebhookSDK{}
	return h, store, stripeClient
}

func signedStripeEventPayload(t *testing.T, eventID string, eventType string, object any) (payload []byte, signature string) {
	t.Helper()

	envelope, err := json.Marshal(map[string]any{
		"id":          eventID,
		"object":      "event",
		"api_version": stripeAPIVersion,
		"type":        eventType,
		"livemode":    false,
		"data": map[string]any{
			"object": object,
		},
	})
	if err != nil {
		t.Fatalf("marshal signed stripe event payload: %v", err)
	}

	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: envelope,
		Secret:  "whsec_test_123",
	})
	return signed.Payload, signed.Header
}

func postSignedStripeWebhook(t *testing.T, h *handler, eventID string, eventType string, object any) *httptest.ResponseRecorder {
	t.Helper()

	payload, signature := signedStripeEventPayload(t, eventID, eventType, object)
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signature)
	w := httptest.NewRecorder()
	h.handleStripeWebhook().ServeHTTP(w, req)
	return w
}

func TestRegisterUsesDedicatedWebhookLimiter(t *testing.T) {
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	apiLimiter := shared.NewIPRateLimiter(0, 0)
	defer apiLimiter.Stop()
	webhookLimiter := shared.NewIPRateLimiter(rate.Limit(10), 10)
	defer webhookLimiter.Stop()

	ctx := &shared.Context{
		Store: store,
		Config: &config.Config{
			CloudHosted:         true,
			StripeSecretKey:     "sk_test_123",
			StripeWebhookSecret: "whsec_test_123",
		},
		ApiLimiter:     apiLimiter,
		WebhookLimiter: webhookLimiter,
	}

	mux := http.NewServeMux()
	Register(mux, ctx)

	payload, signature := signedStripeEventPayload(t, "evt_webhook_limit_ok", "billing.test", map[string]any{
		"id": "obj_test",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signature)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d when webhook limiter allows request, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestRegisterWebhookLimiterCanThrottleStripeWebhook(t *testing.T) {
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	apiLimiter := shared.NewIPRateLimiter(rate.Inf, 1)
	defer apiLimiter.Stop()
	webhookLimiter := shared.NewIPRateLimiter(0, 0)
	defer webhookLimiter.Stop()

	ctx := &shared.Context{
		Store: store,
		Config: &config.Config{
			CloudHosted:         true,
			StripeSecretKey:     "sk_test_123",
			StripeWebhookSecret: "whsec_test_123",
		},
		ApiLimiter:     apiLimiter,
		WebhookLimiter: webhookLimiter,
	}

	mux := http.NewServeMux()
	Register(mux, ctx)

	payload, signature := signedStripeEventPayload(t, "evt_webhook_limit_blocked", "billing.test", map[string]any{
		"id": "obj_test",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signature)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d when webhook limiter blocks request, got %d: %s", http.StatusTooManyRequests, w.Code, w.Body.String())
	}
}

func TestHandleSignupCreatesFreeManagedAccount(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	body, err := json.Marshal(signupRequest{
		Email:        "free@example.com",
		Password:     "password123",
		GivenName:    "Free",
		LastName:     "User",
		TeamName:     "Free Team",
		PlanCode:     database.CloudPlanFree,
		Jurisdiction: "EU",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleSignup().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp signupResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CheckoutURL != "" {
		t.Fatalf("expected free plan to skip checkout, got %q", resp.CheckoutURL)
	}
	if resp.RedirectURL != "/dashboard" {
		t.Fatalf("expected redirect_url /dashboard, got %q", resp.RedirectURL)
	}

	user, err := store.GetUserByEmail(context.Background(), "free@example.com")
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}
	if user == nil {
		t.Fatal("expected created user")
	}

	teams, _, err := store.ListUserTeams(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list user teams: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected one team, got %d", len(teams))
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), teams[0].ID)
	if err != nil {
		t.Fatalf("get billing account: %v", err)
	}
	if billingAccount.PlanCode != database.CloudPlanFree || billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusFree {
		t.Fatalf("unexpected billing account: %+v", billingAccount)
	}
}

func TestHandleSignupStartsStripeCheckoutForPaidPlan(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	body, err := json.Marshal(signupRequest{
		Email:        "pro@example.com",
		Password:     "password123",
		GivenName:    "Pro",
		LastName:     "User",
		TeamName:     "Pro Team",
		PlanCode:     database.CloudPlanPro,
		Jurisdiction: "EU",
		Locale:       "de-DE",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleSignup().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp signupResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CheckoutURL != "https://checkout.stripe.test/session" {
		t.Fatalf("unexpected checkout url %q", resp.CheckoutURL)
	}

	user, err := store.GetUserByEmail(context.Background(), "pro@example.com")
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}
	if user == nil {
		t.Fatal("expected created user")
	}

	teams, _, err := store.ListUserTeams(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list user teams: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected one team, got %d", len(teams))
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), teams[0].ID)
	if err != nil {
		t.Fatalf("get billing account: %v", err)
	}
	if billingAccount.StripeCustomerID != "cus_test" {
		t.Fatalf("unexpected stripe customer id: %+v", billingAccount)
	}
	if billingAccount.PlanCode != database.CloudPlanFree || billingAccount.PlanName != "Free" || billingAccount.SubscriptionStatus != subscriptionStatusPending {
		t.Fatalf("unexpected billing account: %+v", billingAccount)
	}

	stripeClient, ok := h.stripe.(*fakeStripeClient)
	if !ok || stripeClient.lastCheckoutInput == nil {
		t.Fatal("expected checkout session input to be captured")
	}
	if stripeClient.lastCustomerInput == nil {
		t.Fatal("expected customer creation input to be captured")
	}
	if stripeClient.lastCheckoutInput.Locale != "de" {
		t.Fatalf("expected checkout locale de, got %q", stripeClient.lastCheckoutInput.Locale)
	}
	if got, want := stripeClient.lastCustomerInput.IdempotencyKey, stripeCustomerCreateIdempotencyKey(user.ID, teams[0].ID, "pro@example.com"); got != want {
		t.Fatalf("expected customer idempotency key %q, got %q", want, got)
	}
}

func TestHandleStripeEventUpdatesBillingAccount(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "webhook@example.com",
		HashedPassword: "hashed",
		TeamName:       "Webhook Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	event := stripe.Event{
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"metadata":{"tenant_id":"` + account.TenantID.String() + `","plan_code":"pro","plan_name":"Pro"},
				"customer":{"id":"cus_live"},
				"subscription":{"id":"sub_live"},
				"status":"complete"
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), event); err != nil {
		t.Fatalf("handle stripe event: %v", err)
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get billing account: %v", err)
	}
	if billingAccount.StripeCustomerID != "cus_live" || billingAccount.StripeSubscriptionID != "sub_live" {
		t.Fatalf("unexpected billing account: %+v", billingAccount)
	}

	storedEvent, err := store.GetCloudBillingEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get stored cloud billing event: %v", err)
	}
	if storedEvent.ProcessingStatus != database.CloudBillingEventStatusDone {
		t.Fatalf("expected processed billing event, got %+v", storedEvent)
	}
}

func TestHandleStripeEventCheckoutSessionWithoutExpandedRefsDoesNotPanic(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "webhook-minimal@example.com",
		HashedPassword: "hashed",
		TeamName:       "Webhook Minimal Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	event := stripe.Event{
		ID:   "evt_checkout_minimal",
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"metadata":{"tenant_id":"` + account.TenantID.String() + `","plan_code":"pro","plan_name":"Pro"},
				"customer":null,
				"subscription":null,
				"status":"complete"
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), event); err != nil {
		t.Fatalf("handle stripe event: %v", err)
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != "complete" {
		t.Fatalf("expected complete status, got %+v", billingAccount)
	}
	if billingAccount.StripeCustomerID != "" || billingAccount.StripeSubscriptionID != "" {
		t.Fatalf("expected empty stripe refs for minimal session, got %+v", billingAccount)
	}
}

func TestHandleStripeEventIsIdempotent(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "webhook-duplicate@example.com",
		HashedPassword: "hashed",
		TeamName:       "Webhook Duplicate Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	event := stripe.Event{
		ID:   "evt_duplicate",
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"metadata":{"tenant_id":"` + account.TenantID.String() + `","plan_code":"pro","plan_name":"Pro"},
				"customer":{"id":"cus_live"},
				"subscription":{"id":"sub_live"},
				"status":"complete"
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), event); err != nil {
		t.Fatalf("first handle stripe event: %v", err)
	}
	if err := h.handleStripeEvent(context.Background(), event); err != nil {
		t.Fatalf("second handle stripe event: %v", err)
	}

	storedEvent, err := store.GetCloudBillingEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get stored cloud billing event: %v", err)
	}
	if storedEvent.ProcessingStatus != database.CloudBillingEventStatusDone {
		t.Fatalf("expected processed billing event, got %+v", storedEvent)
	}
}

func TestHandleStripeEventIgnoresOtherJurisdictions(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "webhook-us@example.com",
		HashedPassword: "hashed",
		TeamName:       "Webhook US Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	event := stripe.Event{
		ID:   "evt_us_only",
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"metadata":{"tenant_id":"` + account.TenantID.String() + `","plan_code":"pro","plan_name":"Pro","jurisdiction":"US"},
				"customer":{"id":"cus_live"},
				"subscription":{"id":"sub_live"},
				"status":"complete"
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), event); err != nil {
		t.Fatalf("handle stripe event: %v", err)
	}

	if _, err := store.GetCloudBillingEvent(context.Background(), event.ID); !errors.Is(err, database.ErrCloudBillingEventNotFound) {
		t.Fatalf("expected foreign-jurisdiction event to be ignored, got %v", err)
	}
}

func TestHandleStripeEventMarksPaymentFailuresPastDue(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "invoice-failed@example.com",
		HashedPassword: "hashed",
		TeamName:       "Invoice Failed Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:             account.TenantID,
		PlanCode:             database.CloudPlanPro,
		PlanName:             "Pro",
		SubscriptionStatus:   database.CloudSubscriptionStatusActive,
		StripeCustomerID:     "cus_failed",
		StripeSubscriptionID: "sub_failed",
		StripePriceID:        "price_pro",
	}); err != nil {
		t.Fatalf("seed billing account: %v", err)
	}

	event := stripe.Event{
		ID:   "evt_invoice_failed",
		Type: "invoice.payment_failed",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"customer":{"id":"cus_failed"},
				"subscription":{"id":"sub_failed"}
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), event); err != nil {
		t.Fatalf("handle invoice.payment_failed: %v", err)
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusPastDue {
		t.Fatalf("expected past_due status, got %+v", billingAccount)
	}
}

func TestHandleStripeEventMarksDisputes(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "dispute@example.com",
		HashedPassword: "hashed",
		TeamName:       "Dispute Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:             account.TenantID,
		PlanCode:             database.CloudPlanBusiness,
		PlanName:             "Business",
		SubscriptionStatus:   database.CloudSubscriptionStatusActive,
		StripeCustomerID:     "cus_dispute",
		StripeSubscriptionID: "sub_dispute",
		StripePriceID:        "price_business",
	}); err != nil {
		t.Fatalf("seed billing account: %v", err)
	}

	created := stripe.Event{
		ID:   "evt_dispute_created",
		Type: "charge.dispute.created",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"charge":{"id":"ch_dispute"},
				"status":"needs_response"
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), created); err != nil {
		t.Fatalf("handle charge.dispute.created: %v", err)
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get billing account after dispute create: %v", err)
	}
	if billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusDisputed {
		t.Fatalf("expected disputed status, got %+v", billingAccount)
	}

	closed := stripe.Event{
		ID:   "evt_dispute_closed",
		Type: "charge.dispute.closed",
		Data: &stripe.EventData{
			Raw: []byte(`{
				"charge":{"id":"ch_dispute"},
				"status":"lost"
			}`),
		},
	}

	if err := h.handleStripeEvent(context.Background(), closed); err != nil {
		t.Fatalf("handle charge.dispute.closed: %v", err)
	}

	billingAccount, err = store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get billing account after dispute close: %v", err)
	}
	if billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusChargebackLost {
		t.Fatalf("expected chargeback_lost status, got %+v", billingAccount)
	}

	stripeClient, ok := h.stripe.(*fakeStripeClient)
	if !ok {
		t.Fatal("expected fake stripe client")
	}
	if stripeClient.lastChargeID != "ch_dispute" {
		t.Fatalf("expected disputed charge lookup, got %q", stripeClient.lastChargeID)
	}
}

func TestHandleStripeWebhookSignedSubscriptionLifecycle(t *testing.T) {
	h, store, _ := setupSignedCloudWebhookTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "signed-webhook@example.com",
		HashedPassword: "hashed",
		TeamName:       "Signed Webhook Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:           account.TenantID,
		PlanCode:           database.CloudPlanPro,
		PlanName:           "Pro",
		SubscriptionStatus: subscriptionStatusPending,
		StripeCustomerID:   "cus_live",
		StripePriceID:      "price_pro",
	}); err != nil {
		t.Fatalf("seed billing account: %v", err)
	}

	activate := postSignedStripeWebhook(t, h, "evt_sub_updated", "customer.subscription.updated", map[string]any{
		"id": "sub_live",
		"metadata": map[string]string{
			"tenant_id":    account.TenantID.String(),
			"plan_code":    database.CloudPlanPro,
			"plan_name":    "Pro",
			"jurisdiction": "EU",
		},
		"status":   subscriptionStatusActive,
		"customer": map[string]string{"id": "cus_live"},
		"items": map[string]any{
			"data": []map[string]any{
				{"price": map[string]string{"id": "price_pro"}},
			},
		},
	})
	if activate.Code != http.StatusOK {
		t.Fatalf("expected activation webhook status %d, got %d: %s", http.StatusOK, activate.Code, activate.Body.String())
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get activated billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != subscriptionStatusActive {
		t.Fatalf("expected active subscription status, got %+v", billingAccount)
	}
	if billingAccount.StripeSubscriptionID != "sub_live" || billingAccount.StripePriceID != "price_pro" {
		t.Fatalf("expected subscription identifiers to be stored, got %+v", billingAccount)
	}

	replayed := postSignedStripeWebhook(t, h, "evt_sub_updated", "customer.subscription.updated", map[string]any{
		"id": "sub_live",
		"metadata": map[string]string{
			"tenant_id": account.TenantID.String(),
			"plan_code": database.CloudPlanPro,
			"plan_name": "Pro",
		},
		"status":   subscriptionStatusActive,
		"customer": map[string]string{"id": "cus_live"},
		"items": map[string]any{
			"data": []map[string]any{
				{"price": map[string]string{"id": "price_pro"}},
			},
		},
	})
	if replayed.Code != http.StatusOK {
		t.Fatalf("expected replayed webhook status %d, got %d: %s", http.StatusOK, replayed.Code, replayed.Body.String())
	}

	failedPayment := postSignedStripeWebhook(t, h, "evt_invoice_failed_signed", "invoice.payment_failed", map[string]any{
		"customer":     map[string]string{"id": "cus_live"},
		"subscription": map[string]string{"id": "sub_live"},
		"metadata": map[string]string{
			"tenant_id": account.TenantID.String(),
		},
	})
	if failedPayment.Code != http.StatusOK {
		t.Fatalf("expected invoice.payment_failed webhook status %d, got %d: %s", http.StatusOK, failedPayment.Code, failedPayment.Body.String())
	}

	billingAccount, err = store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get past-due billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusPastDue {
		t.Fatalf("expected past_due status, got %+v", billingAccount)
	}

	storedEvent, err := store.GetCloudBillingEvent(context.Background(), "evt_sub_updated")
	if err != nil {
		t.Fatalf("get replayed billing event: %v", err)
	}
	if storedEvent.ProcessingStatus != database.CloudBillingEventStatusDone {
		t.Fatalf("expected replayed billing event to remain processed, got %+v", storedEvent)
	}
}

func TestHandleStripeWebhookSignedChargeDisputeLifecycle(t *testing.T) {
	h, store, stripeClient := setupSignedCloudWebhookTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "signed-dispute@example.com",
		HashedPassword: "hashed",
		TeamName:       "Signed Dispute Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	stripeClient.chargeCustomerID = "cus_dispute_signed"
	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:             account.TenantID,
		PlanCode:             database.CloudPlanBusiness,
		PlanName:             "Business",
		SubscriptionStatus:   database.CloudSubscriptionStatusActive,
		StripeCustomerID:     "cus_dispute_signed",
		StripeSubscriptionID: "sub_dispute_signed",
		StripePriceID:        "price_business",
	}); err != nil {
		t.Fatalf("seed billing account: %v", err)
	}

	disputed := postSignedStripeWebhook(t, h, "evt_dispute_created_signed", "charge.dispute.created", map[string]any{
		"charge": map[string]string{"id": "ch_dispute_signed"},
		"status": "needs_response",
	})
	if disputed.Code != http.StatusOK {
		t.Fatalf("expected dispute.created webhook status %d, got %d: %s", http.StatusOK, disputed.Code, disputed.Body.String())
	}

	billingAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get disputed billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusDisputed {
		t.Fatalf("expected disputed status, got %+v", billingAccount)
	}

	lost := postSignedStripeWebhook(t, h, "evt_dispute_closed_lost_signed", "charge.dispute.closed", map[string]any{
		"charge": map[string]string{"id": "ch_dispute_signed"},
		"status": "lost",
	})
	if lost.Code != http.StatusOK {
		t.Fatalf("expected dispute.closed lost webhook status %d, got %d: %s", http.StatusOK, lost.Code, lost.Body.String())
	}

	billingAccount, err = store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get lost-dispute billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != database.CloudSubscriptionStatusChargebackLost {
		t.Fatalf("expected chargeback_lost status, got %+v", billingAccount)
	}

	won := postSignedStripeWebhook(t, h, "evt_dispute_closed_won_signed", "charge.dispute.closed", map[string]any{
		"charge": map[string]string{"id": "ch_dispute_signed"},
		"status": "won",
	})
	if won.Code != http.StatusOK {
		t.Fatalf("expected dispute.closed won webhook status %d, got %d: %s", http.StatusOK, won.Code, won.Body.String())
	}

	billingAccount, err = store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get won-dispute billing account: %v", err)
	}
	if billingAccount.SubscriptionStatus != subscriptionStatusActive {
		t.Fatalf("expected active status after won dispute, got %+v", billingAccount)
	}

	if stripeClient.lastChargeID != "ch_dispute_signed" {
		t.Fatalf("expected signed dispute charge lookup, got %q", stripeClient.lastChargeID)
	}
}

func TestStripeWebhookSDKUsesConfiguredAPIVersion(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id":"evt_test_configured_api",
		"object":"event",
		"api_version":"2026-02-25.clover",
		"type":"invoice.payment_failed",
		"data":{"object":{"customer":"cus_test","subscription":"sub_test"}}
	}`)

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("whsec_test_123"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	signature := fmt.Sprintf("t=%s,v1=%x", timestamp, mac.Sum(nil))

	event, err := stripeWebhookSDK{}.ConstructEvent(payload, signature, "whsec_test_123")
	if err != nil {
		t.Fatalf("construct event: %v", err)
	}
	if event.Type != "invoice.payment_failed" {
		t.Fatalf("expected invoice.payment_failed, got %q", event.Type)
	}
	if !strings.Contains(string(event.Data.Raw), `"customer":"cus_test"`) {
		t.Fatalf("expected raw invoice payload to be preserved, got %s", string(event.Data.Raw))
	}
	if event.APIVersion != stripeAPIVersion {
		t.Fatalf("expected event api version %q, got %q", stripeAPIVersion, event.APIVersion)
	}
}

func TestStripeCustomerIDHandlesNil(t *testing.T) {
	t.Parallel()

	if got := stripeCustomerID(nil); got != "" {
		t.Fatalf("expected empty customer id for nil customer, got %q", got)
	}
	if got := stripeCustomerID(&stripe.Customer{ID: "cus_test"}); got != "cus_test" {
		t.Fatalf("expected cus_test, got %q", got)
	}
}

func TestSetStripeVersionHeader(t *testing.T) {
	t.Parallel()

	params := &stripe.Params{}
	setStripeVersionHeader(params)

	if got := params.Headers.Get("Stripe-Version"); got != stripeAPIVersion {
		t.Fatalf("expected stripe version header %q, got %q", stripeAPIVersion, got)
	}
}

func TestSetStripeIdempotencyKey(t *testing.T) {
	t.Parallel()

	params := &stripe.Params{}
	setStripeIdempotencyKey(params, " hitkeep:test:key ")

	if params.IdempotencyKey == nil {
		t.Fatal("expected idempotency key to be set")
	}
	if got := *params.IdempotencyKey; got != "hitkeep:test:key" {
		t.Fatalf("expected trimmed idempotency key, got %q", got)
	}
}

func TestHandleCreateBillingPortalSession(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "portal@example.com",
		HashedPassword: "hashed",
		TeamName:       "Portal Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:           account.TenantID,
		PlanCode:           database.CloudPlanPro,
		PlanName:           "Pro",
		SubscriptionStatus: "active",
		StripeCustomerID:   "cus_portal",
	}); err != nil {
		t.Fatalf("upsert cloud billing account: %v", err)
	}

	body, err := json.Marshal(billingPortalSessionRequest{Locale: "fr-FR"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/billing/portal", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, account.UserID))
	w := httptest.NewRecorder()

	h.handleCreateBillingPortalSession().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp billingPortalSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.URL != "https://billing.stripe.test/session" {
		t.Fatalf("unexpected billing portal url %q", resp.URL)
	}

	stripeClient, ok := h.stripe.(*fakeStripeClient)
	if !ok || stripeClient.lastPortalInput == nil {
		t.Fatal("expected billing portal input to be captured")
	}
	if stripeClient.lastPortalInput.Locale != "fr" {
		t.Fatalf("expected billing portal locale fr, got %q", stripeClient.lastPortalInput.Locale)
	}
}

func TestHandleCreateBillingCheckoutSession(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "upgrade@example.com",
		HashedPassword: "hashed",
		GivenName:      "Ada",
		LastName:       "Lovelace",
		TeamName:       "Upgrade Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:           account.TenantID,
		PlanCode:           database.CloudPlanFree,
		PlanName:           "Free",
		SubscriptionStatus: database.CloudSubscriptionStatusFree,
	}); err != nil {
		t.Fatalf("upsert cloud billing account: %v", err)
	}

	body, err := json.Marshal(billingCheckoutSessionRequest{PlanCode: database.CloudPlanPro, Locale: "de-DE"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/billing/checkout", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, account.UserID))
	w := httptest.NewRecorder()

	h.handleCreateBillingCheckoutSession().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp billingCheckoutSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.URL != "https://checkout.stripe.test/session" {
		t.Fatalf("unexpected billing checkout url %q", resp.URL)
	}

	stripeClient, ok := h.stripe.(*fakeStripeClient)
	if !ok {
		t.Fatal("expected fake stripe client")
	}
	if stripeClient.lastCustomerInput == nil {
		t.Fatal("expected stripe customer create input to be captured")
	}
	if stripeClient.lastCustomerInput.Email != "upgrade@example.com" {
		t.Fatalf("expected customer email upgrade@example.com, got %q", stripeClient.lastCustomerInput.Email)
	}
	if stripeClient.lastCheckoutInput == nil {
		t.Fatal("expected stripe checkout input to be captured")
	}
	if stripeClient.lastCheckoutInput.Locale != "de" {
		t.Fatalf("expected checkout locale de, got %q", stripeClient.lastCheckoutInput.Locale)
	}
	if stripeClient.lastCheckoutInput.PlanCode != database.CloudPlanPro {
		t.Fatalf("expected checkout plan %q, got %q", database.CloudPlanPro, stripeClient.lastCheckoutInput.PlanCode)
	}

	storedAccount, err := store.GetCloudBillingAccount(context.Background(), account.TenantID)
	if err != nil {
		t.Fatalf("get cloud billing account: %v", err)
	}
	if storedAccount.SubscriptionStatus != subscriptionStatusPending {
		t.Fatalf("expected subscription status %q, got %q", subscriptionStatusPending, storedAccount.SubscriptionStatus)
	}
	if storedAccount.PlanCode != database.CloudPlanFree {
		t.Fatalf("expected persisted plan code %q, got %q", database.CloudPlanFree, storedAccount.PlanCode)
	}
	if storedAccount.StripeCustomerID != "cus_test" {
		t.Fatalf("expected persisted customer id cus_test, got %q", storedAccount.StripeCustomerID)
	}
	if storedAccount.StripePriceID != "price_pro" {
		t.Fatalf("expected persisted price id price_pro, got %q", storedAccount.StripePriceID)
	}
}

func TestHandleCreateBillingCheckoutSessionRejectsPaidTeams(t *testing.T) {
	h, store := setupCloudTestHandler(t)
	defer store.Close()

	account, err := store.CreateManagedCloudAccount(context.Background(), database.CreateManagedCloudAccountInput{
		Email:          "paid@example.com",
		HashedPassword: "hashed",
		TeamName:       "Paid Team",
	})
	if err != nil {
		t.Fatalf("create managed account: %v", err)
	}

	if err := store.UpsertCloudBillingAccount(context.Background(), database.CloudBillingAccount{
		TenantID:           account.TenantID,
		PlanCode:           database.CloudPlanPro,
		PlanName:           "Pro",
		SubscriptionStatus: subscriptionStatusActive,
		StripeCustomerID:   "cus_existing",
	}); err != nil {
		t.Fatalf("upsert cloud billing account: %v", err)
	}

	body, err := json.Marshal(billingCheckoutSessionRequest{PlanCode: database.CloudPlanBusiness})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/billing/checkout", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, account.UserID))
	w := httptest.NewRecorder()

	h.handleCreateBillingCheckoutSession().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

func TestNormalizeStripeLocale(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty falls back to auto", in: "", want: "auto"},
		{name: "base locale passes through", in: "de", want: "de"},
		{name: "region locale maps to base", in: "fr-FR", want: "fr"},
		{name: "supported regional locale preserved", in: "pt-BR", want: "pt-BR"},
		{name: "unsupported locale falls back to auto", in: "ga-IE", want: "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeStripeLocale(tt.in); got != tt.want {
				t.Fatalf("normalizeStripeLocale(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
