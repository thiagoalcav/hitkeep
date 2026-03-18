//go:build billing

package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"

	"hitkeep/internal/api"
	authcore "hitkeep/internal/auth"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/mailables"
	serverauth "hitkeep/internal/server/auth"
	"hitkeep/internal/server/shared"
)

const (
	checkoutModeSubscription   = "subscription"
	subscriptionStatusActive   = "active"
	subscriptionStatusPending  = "pending_checkout"
	subscriptionStatusCanceled = "canceled"
	stripeAPIVersion           = "2026-02-25.clover"
)

type stripeClient interface {
	CreateCustomer(context.Context, createCustomerInput) (string, error)
	CreateCheckoutSession(context.Context, createCheckoutSessionInput) (*checkoutSessionOutput, error)
	CreatePortalSession(context.Context, createPortalSessionInput) (*portalSessionOutput, error)
	GetCharge(context.Context, string) (*stripeChargeOutput, error)
}

type stripeWebhookVerifier interface {
	ConstructEvent(payload []byte, header string, secret string) (stripe.Event, error)
}

type handler struct {
	ctx      *shared.Context
	stripe   stripeClient
	webhooks stripeWebhookVerifier
}

type createCustomerInput struct {
	Email          string
	Name           string
	UserID         uuid.UUID
	TenantID       uuid.UUID
	PlanCode       string
	Jurisdiction   string
	IdempotencyKey string
}

type createCheckoutSessionInput struct {
	CustomerID   string
	PriceID      string
	SuccessURL   string
	CancelURL    string
	Locale       string
	UserID       uuid.UUID
	TenantID     uuid.UUID
	PlanCode     string
	PlanName     string
	Jurisdiction string
	Email        string
}

type checkoutSessionOutput struct {
	ID  string
	URL string
}

type createPortalSessionInput struct {
	CustomerID      string
	ConfigurationID string
	ReturnURL       string
	Locale          string
}

type portalSessionOutput struct {
	ID  string
	URL string
}

type stripeChargeOutput struct {
	ID         string
	CustomerID string
}

type stripeSDKClient struct {
	client *stripe.Client
}

type stripeWebhookSDK struct{}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{
		ctx:      ctx,
		stripe:   newStripeSDKClient(ctx.Config.StripeSecretKey),
		webhooks: stripeWebhookSDK{},
	}

	mux.HandleFunc("POST /api/cloud/signup", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleSignup()))
	mux.HandleFunc("GET /api/cloud/signup/verify", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.AuthLimiter,
	}, h.handleVerifySignup()))
	mux.HandleFunc("POST /api/cloud/billing/portal", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleCreateBillingPortalSession()))
	mux.HandleFunc("POST /api/cloud/billing/checkout", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleCreateBillingCheckoutSession()))
	mux.HandleFunc("GET /api/cloud/plans", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.ApiLimiter,
	}, h.handleListCloudPlans()))
	mux.HandleFunc("POST /api/cloud/webhooks/stripe", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.WebhookLimiter,
	}, h.handleStripeWebhook()))
}

type signupRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	GivenName    string `json:"given_name"`
	LastName     string `json:"last_name"`
	TeamName     string `json:"team_name"`
	PlanCode     string `json:"plan_code"`
	Jurisdiction string `json:"jurisdiction"`
	Locale       string `json:"locale"`
	AcceptedTos  bool   `json:"accepted_tos"`
}

type signupResponse struct {
	Status      string `json:"status"`
	PlanCode    string `json:"plan_code"`
	RedirectURL string `json:"redirect_url,omitempty"`
	CheckoutURL string `json:"checkout_url,omitempty"`
}

type billingPortalSessionResponse struct {
	URL string `json:"url"`
}

type billingPortalSessionRequest struct {
	Locale string `json:"locale"`
}

type billingCheckoutSessionRequest struct {
	PlanCode string `json:"plan_code"`
	Locale   string `json:"locale"`
}

type billingCheckoutSessionResponse struct {
	URL string `json:"url"`
}

func (h *handler) handleSignup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ctx.Config.CloudHosted || !h.ctx.Config.CloudSignupEnabled {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		var req signupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.GivenName = strings.TrimSpace(req.GivenName)
		req.LastName = strings.TrimSpace(req.LastName)
		req.TeamName = strings.TrimSpace(req.TeamName)
		req.PlanCode = database.CloudPlanFree // signup always starts on free
		req.Jurisdiction = strings.TrimSpace(strings.ToUpper(req.Jurisdiction))
		req.Locale = normalizeStripeLocale(req.Locale)

		if req.Email == "" || len(req.Password) < 8 {
			http.Error(w, "Email required; Password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		if req.TeamName == "" {
			http.Error(w, "Team name is required", http.StatusBadRequest)
			return
		}
		if !req.AcceptedTos {
			http.Error(w, "You must accept the Terms of Service and Privacy Policy", http.StatusBadRequest)
			return
		}
		if configuredJurisdiction := normalizeJurisdiction(h.ctx.Config.CloudJurisdiction); configuredJurisdiction != "" && req.Jurisdiction != "" && normalizeJurisdiction(req.Jurisdiction) != configuredJurisdiction {
			http.Error(w, "Jurisdiction mismatch", http.StatusBadRequest)
			return
		}

		hashedPassword, err := serverauth.HashPassword(req.Password)
		if err != nil {
			slog.Error("Failed to hash cloud signup password", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		existing, _ := h.ctx.Store.GetUserByEmail(r.Context(), req.Email)
		if existing != nil {
			http.Error(w, "Email already exists", http.StatusConflict)
			return
		}

		token, err := h.ctx.Store.CreatePendingSignup(r.Context(), database.PendingSignupEntry{
			Email:          req.Email,
			HashedPassword: hashedPassword,
			GivenName:      req.GivenName,
			LastName:       req.LastName,
			TeamName:       req.TeamName,
			Jurisdiction:   req.Jurisdiction,
			Locale:         req.Locale,
			AcceptedTosAt:  time.Now().UTC(),
		})
		if err != nil {
			slog.Error("Failed to create pending signup token", "error", err, "email", req.Email)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		verifyLink := strings.TrimRight(h.ctx.Config.PublicURL, "/") + "/api/cloud/signup/verify?token=" + token
		if err := h.ctx.Mailer.Send(req.Email, mailables.NewEmailVerification(verifyLink, req.TeamName)); err != nil {
			slog.Error("Failed to send verification email", "error", err, "email", req.Email)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resp := signupResponse{
			Status:   "verification_sent",
			PlanCode: database.CloudPlanFree,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode cloud signup response", "error", err)
		}
	}
}

func (h *handler) handleVerifySignup() http.HandlerFunc {
	signupURL := func(errorCode string) string {
		return strings.TrimRight(h.ctx.Config.PublicURL, "/") + "/signup?error=" + errorCode
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ctx.Config.CloudHosted || !h.ctx.Config.CloudSignupEnabled {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			http.Redirect(w, r, signupURL("expired"), http.StatusFound)
			return
		}

		entry, err := h.ctx.Store.CompletePendingSignup(r.Context(), token)
		if err != nil {
			slog.Warn("Signup verification failed", "error", err, "token_prefix", safeTokenPrefix(token))
			http.Redirect(w, r, signupURL("expired"), http.StatusFound)
			return
		}

		account, err := h.ctx.Store.CreateManagedCloudAccount(r.Context(), database.CreateManagedCloudAccountInput{
			Email:          entry.Email,
			HashedPassword: entry.HashedPassword,
			GivenName:      entry.GivenName,
			LastName:       entry.LastName,
			TeamName:       entry.TeamName,
		})
		if errors.Is(err, database.ErrUserEmailAlreadyExists) {
			http.Redirect(w, r, signupURL("exists"), http.StatusFound)
			return
		}
		if err != nil {
			slog.Error("Failed to create managed cloud account during verification", "error", err, "email", entry.Email)
			http.Redirect(w, r, signupURL("expired"), http.StatusFound)
			return
		}

		if err := h.ctx.Store.UpsertCloudBillingAccount(r.Context(), database.CloudBillingAccount{
			TenantID:           account.TenantID,
			PlanCode:           database.CloudPlanFree,
			PlanName:           planNameForCode(database.CloudPlanFree),
			SubscriptionStatus: database.CloudSubscriptionStatusFree,
		}); err != nil {
			slog.Error("Failed to initialize cloud billing account", "error", err, "team_id", account.TenantID)
			http.Redirect(w, r, signupURL("expired"), http.StatusFound)
			return
		}

		if err := issueLoginSession(w, h.ctx.Config, account.UserID); err != nil {
			slog.Error("Failed to issue cloud signup login session", "error", err, "user_id", account.UserID)
			http.Redirect(w, r, signupURL("expired"), http.StatusFound)
			return
		}

		dashboardURL := strings.TrimRight(h.ctx.Config.PublicURL, "/") + "/dashboard"
		http.Redirect(w, r, dashboardURL, http.StatusFound)
	}
}

func (h *handler) handleListCloudPlans() http.HandlerFunc {
	plans := []api.CloudPlanTier{
		{
			Code: database.CloudPlanFree,
			Name: planNameForCode(database.CloudPlanFree),
			Entitlements: api.TeamEntitlements{
				MaxSitesPerTeam:     3,
				MaxTeamMembers:      3,
				MaxRetentionDays:    60,
				AllowSSO:            false,
				AllowCustomBranding: false,
			},
		},
		{
			Code: database.CloudPlanPro,
			Name: planNameForCode(database.CloudPlanPro),
			Entitlements: api.TeamEntitlements{
				MaxSitesPerTeam:     10,
				MaxTeamMembers:      5,
				MaxRetentionDays:    365,
				AllowSSO:            false,
				AllowCustomBranding: false,
			},
		},
		{
			Code: database.CloudPlanBusiness,
			Name: planNameForCode(database.CloudPlanBusiness),
			Entitlements: api.TeamEntitlements{
				MaxSitesPerTeam:     50,
				MaxTeamMembers:      20,
				MaxRetentionDays:    1095,
				AllowSSO:            true,
				AllowCustomBranding: true,
			},
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ctx.Config.CloudHosted {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(plans); err != nil {
			slog.Error("Failed to encode cloud plans response", "error", err)
		}
	}
}

func safeTokenPrefix(token string) string {
	if len(token) > 8 {
		return token[:8] + "..."
	}
	return "***"
}

func (h *handler) createCheckout(ctx context.Context, account *database.ManagedCloudAccount, req signupRequest) (checkoutURL string, customerID string, sessionID string, priceID string, err error) {
	planName := planNameForCode(req.PlanCode)
	priceID = priceIDForPlan(h.ctx.Config, req.PlanCode)
	if priceID == "" {
		return "", "", "", "", fmt.Errorf("plan %s is not configured for checkout", req.PlanCode)
	}
	if strings.TrimSpace(h.ctx.Config.StripeSecretKey) == "" {
		return "", "", "", "", fmt.Errorf("stripe secret key is not configured")
	}

	displayName := strings.TrimSpace(strings.Join([]string{req.GivenName, req.LastName}, " "))
	customerID, err = h.stripe.CreateCustomer(ctx, createCustomerInput{
		Email:          req.Email,
		Name:           displayName,
		UserID:         account.UserID,
		TenantID:       account.TenantID,
		PlanCode:       req.PlanCode,
		Jurisdiction:   effectiveJurisdiction(h.ctx.Config, req.Jurisdiction),
		IdempotencyKey: stripeCustomerCreateIdempotencyKey(account.UserID, account.TenantID, req.Email),
	})
	if err != nil {
		return "", "", "", "", err
	}

	session, err := h.stripe.CreateCheckoutSession(ctx, createCheckoutSessionInput{
		CustomerID:   customerID,
		PriceID:      priceID,
		SuccessURL:   checkoutSuccessURL(h.ctx.Config),
		CancelURL:    checkoutCancelURL(h.ctx.Config),
		Locale:       normalizeStripeLocale(req.Locale),
		UserID:       account.UserID,
		TenantID:     account.TenantID,
		PlanCode:     req.PlanCode,
		PlanName:     planName,
		Jurisdiction: effectiveJurisdiction(h.ctx.Config, req.Jurisdiction),
		Email:        req.Email,
	})
	if err != nil {
		return "", "", "", "", err
	}

	return session.URL, customerID, session.ID, priceID, nil
}

func (h *handler) handleStripeWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ctx.Config.CloudHosted {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if strings.TrimSpace(h.ctx.Config.StripeWebhookSecret) == "" {
			http.Error(w, "Webhook signing secret not configured", http.StatusNotImplemented)
			return
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			return
		}

		event, err := h.webhooks.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.ctx.Config.StripeWebhookSecret)
		if err != nil {
			http.Error(w, "Invalid webhook signature", http.StatusBadRequest)
			return
		}

		if err := h.handleStripeEvent(r.Context(), event); err != nil {
			slog.Error("Failed to process Stripe webhook", "error", err, "type", event.Type, "event_id", event.ID)
			http.Error(w, "Webhook processing failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (h *handler) handleCreateBillingPortalSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ctx.Config.CloudHosted {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		activeTenantID, err := h.ctx.Store.GetActiveTenantID(r.Context(), userID)
		if err != nil {
			http.Error(w, "Unable to resolve active team", http.StatusBadRequest)
			return
		}

		account, err := h.ctx.Store.GetCloudBillingAccount(r.Context(), activeTenantID)
		if errors.Is(err, database.ErrCloudBillingAccountNotFound) || account == nil {
			newAccount := database.CloudBillingAccount{
				TenantID:           activeTenantID,
				PlanCode:           database.CloudPlanFree,
				PlanName:           planNameForCode(database.CloudPlanFree),
				SubscriptionStatus: database.CloudSubscriptionStatusFree,
			}
			if upsertErr := h.ctx.Store.UpsertCloudBillingAccount(r.Context(), newAccount); upsertErr != nil {
				slog.Error("Failed to auto-create cloud billing account", "error", upsertErr, "team_id", activeTenantID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			account = &newAccount
		}
		if err != nil && !errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			slog.Error("Failed to load cloud billing account", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(account.StripeCustomerID) == "" {
			http.Error(w, "Stripe customer is not configured", http.StatusConflict)
			return
		}
		if strings.TrimSpace(h.ctx.Config.StripeSecretKey) == "" {
			http.Error(w, "Stripe secret key is not configured", http.StatusNotImplemented)
			return
		}

		var req billingPortalSessionRequest
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
		}

		session, err := h.stripe.CreatePortalSession(r.Context(), createPortalSessionInput{
			CustomerID:      account.StripeCustomerID,
			ConfigurationID: strings.TrimSpace(h.ctx.Config.StripePortalConfigurationID),
			ReturnURL:       billingPortalReturnURL(h.ctx.Config),
			Locale:          normalizeStripeLocale(req.Locale),
		})
		if err != nil {
			slog.Error("Failed to create Stripe billing portal session", "error", err)
			http.Error(w, "Unable to start billing portal", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(billingPortalSessionResponse{URL: session.URL}); err != nil {
			slog.Error("Failed to encode billing portal session response", "error", err)
		}
	}
}

func (h *handler) handleCreateBillingCheckoutSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ctx.Config.CloudHosted {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		activeTenantID, err := h.ctx.Store.GetActiveTenantID(r.Context(), userID)
		if err != nil {
			http.Error(w, "Unable to resolve active team", http.StatusBadRequest)
			return
		}

		account, err := h.ctx.Store.GetCloudBillingAccount(r.Context(), activeTenantID)
		if errors.Is(err, database.ErrCloudBillingAccountNotFound) || account == nil {
			// Auto-create a free billing account for legacy users who signed up
			// before the billing account table existed.
			newAccount := database.CloudBillingAccount{
				TenantID:           activeTenantID,
				PlanCode:           database.CloudPlanFree,
				PlanName:           planNameForCode(database.CloudPlanFree),
				SubscriptionStatus: database.CloudSubscriptionStatusFree,
			}
			if upsertErr := h.ctx.Store.UpsertCloudBillingAccount(r.Context(), newAccount); upsertErr != nil {
				slog.Error("Failed to auto-create cloud billing account", "error", upsertErr, "team_id", activeTenantID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			account = &newAccount
		}
		if err != nil && !errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			slog.Error("Failed to load cloud billing account", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(h.ctx.Config.StripeSecretKey) == "" {
			http.Error(w, "Stripe secret key is not configured", http.StatusNotImplemented)
			return
		}

		var req billingCheckoutSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		req.PlanCode = normalizePlanCode(req.PlanCode)
		req.Locale = normalizeStripeLocale(req.Locale)
		if req.PlanCode != database.CloudPlanPro && req.PlanCode != database.CloudPlanBusiness {
			http.Error(w, "Paid plan code is required", http.StatusBadRequest)
			return
		}

		currentPlanCode, _ := effectivePlanCode(account)
		if currentPlanCode != database.CloudPlanFree {
			http.Error(w, "Use billing portal to manage an existing paid plan", http.StatusConflict)
			return
		}

		user, err := h.ctx.Store.GetUserByID(r.Context(), userID)
		if err != nil || user == nil {
			slog.Error("Failed to load user for billing checkout", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		team, err := h.ctx.Store.GetTenant(r.Context(), activeTenantID)
		if err != nil || team == nil {
			slog.Error("Failed to load team for billing checkout", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		customerID := strings.TrimSpace(account.StripeCustomerID)
		if customerID == "" {
			customerID, err = h.stripe.CreateCustomer(r.Context(), createCustomerInput{
				Email:          strings.TrimSpace(strings.ToLower(user.Email)),
				Name:           stripeCustomerName(user, team.Name),
				UserID:         user.ID,
				TenantID:       activeTenantID,
				PlanCode:       req.PlanCode,
				Jurisdiction:   strings.TrimSpace(strings.ToUpper(h.ctx.Config.CloudJurisdiction)),
				IdempotencyKey: stripeCustomerCreateIdempotencyKey(user.ID, activeTenantID, user.Email),
			})
			if err != nil {
				slog.Error("Failed to create Stripe customer for billing checkout", "error", err)
				http.Error(w, "Unable to start checkout", http.StatusBadGateway)
				return
			}
		}

		priceID := priceIDForPlan(h.ctx.Config, req.PlanCode)
		if priceID == "" {
			http.Error(w, "Plan is not configured for checkout", http.StatusBadRequest)
			return
		}

		session, err := h.stripe.CreateCheckoutSession(r.Context(), createCheckoutSessionInput{
			CustomerID:   customerID,
			PriceID:      priceID,
			SuccessURL:   checkoutSuccessURL(h.ctx.Config),
			CancelURL:    checkoutCancelURL(h.ctx.Config),
			Locale:       req.Locale,
			UserID:       user.ID,
			TenantID:     activeTenantID,
			PlanCode:     req.PlanCode,
			PlanName:     planNameForCode(req.PlanCode),
			Jurisdiction: strings.TrimSpace(strings.ToUpper(h.ctx.Config.CloudJurisdiction)),
			Email:        strings.TrimSpace(strings.ToLower(user.Email)),
		})
		if err != nil {
			slog.Error("Failed to create Stripe upgrade checkout session", "error", err)
			http.Error(w, "Unable to start checkout", http.StatusBadGateway)
			return
		}

		if err := h.ctx.Store.UpsertCloudBillingAccount(r.Context(), database.CloudBillingAccount{
			TenantID:             activeTenantID,
			PlanCode:             database.CloudPlanFree,
			PlanName:             planNameForCode(database.CloudPlanFree),
			SubscriptionStatus:   subscriptionStatusPending,
			StripeCustomerID:     customerID,
			StripeSubscriptionID: "",
			StripePriceID:        priceID,
		}); err != nil {
			slog.Error("Failed to persist Stripe upgrade checkout metadata", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(billingCheckoutSessionResponse{URL: session.URL}); err != nil {
			slog.Error("Failed to encode billing checkout session response", "error", err)
		}
	}
}

func (h *handler) handleStripeEvent(ctx context.Context, event stripe.Event) error {
	eventJurisdiction, err := jurisdictionFromStripeEvent(event)
	if err != nil {
		return err
	}
	configuredJurisdiction := strings.TrimSpace(strings.ToUpper(h.ctx.Config.CloudJurisdiction))
	if configuredJurisdiction != "" && eventJurisdiction != "" && eventJurisdiction != configuredJurisdiction {
		return nil
	}

	tenantID, _ := tenantIDFromStripeEvent(event)
	inserted, err := h.ctx.Store.CreateCloudBillingEvent(ctx, database.CloudBillingEvent{
		StripeEventID: event.ID,
		TenantID:      tenantID,
		EventType:     string(event.Type),
		Livemode:      event.Livemode,
		Payload:       string(event.Data.Raw),
	})
	if err != nil {
		return fmt.Errorf("record stripe event: %w", err)
	}
	if !inserted {
		storedEvent, lookupErr := h.ctx.Store.GetCloudBillingEvent(ctx, event.ID)
		if lookupErr != nil {
			return fmt.Errorf("load existing stripe event: %w", lookupErr)
		}
		if storedEvent.ProcessingStatus == database.CloudBillingEventStatusDone {
			return nil
		}
	}

	updateEventStatus := func(status string, processedTenantID uuid.UUID, processErr error) error {
		billingEvent := database.CloudBillingEvent{
			StripeEventID:    event.ID,
			TenantID:         processedTenantID,
			ProcessingStatus: status,
		}
		if processErr != nil {
			billingEvent.ProcessingError = processErr.Error()
		} else {
			processedAt := stripeEventProcessedAt()
			billingEvent.ProcessedAt = &processedAt
		}
		return h.ctx.Store.UpdateCloudBillingEventStatus(ctx, billingEvent)
	}

	var handleErr error
	switch string(event.Type) {
	case "checkout.session.completed":
		session, err := parseStripeCheckoutSessionEvent(event.Data.Raw)
		if err != nil {
			handleErr = fmt.Errorf("decode checkout.session.completed: %w", err)
			break
		}
		tenantID, err := uuid.Parse(strings.TrimSpace(session.Metadata["tenant_id"]))
		if err != nil {
			handleErr = fmt.Errorf("parse checkout tenant_id: %w", err)
			break
		}
		handleErr = h.ctx.Store.UpsertCloudBillingAccount(ctx, database.CloudBillingAccount{
			TenantID:             tenantID,
			PlanCode:             normalizePlanCode(session.Metadata["plan_code"]),
			PlanName:             strings.TrimSpace(session.Metadata["plan_name"]),
			SubscriptionStatus:   session.Status,
			StripeCustomerID:     session.Customer.ID,
			StripeSubscriptionID: session.Subscription.ID,
			StripePriceID:        session.FirstPriceID(),
		})
	case "customer.subscription.updated", "customer.subscription.deleted":
		subscription, err := parseStripeSubscriptionEvent(event.Data.Raw)
		if err != nil {
			handleErr = fmt.Errorf("decode subscription event: %w", err)
			break
		}
		tenantID, err := tenantIDFromStripeMetadata(subscription.Metadata)
		if err != nil {
			handleErr = err
			break
		}
		handleErr = h.ctx.Store.UpsertCloudBillingAccount(ctx, database.CloudBillingAccount{
			TenantID:             tenantID,
			PlanCode:             normalizePlanCode(subscription.Metadata["plan_code"]),
			PlanName:             strings.TrimSpace(subscription.Metadata["plan_name"]),
			SubscriptionStatus:   subscription.Status,
			StripeCustomerID:     subscription.Customer.ID,
			StripeSubscriptionID: subscription.ID,
			StripePriceID:        subscription.FirstPriceID(),
		})
	case "invoice.payment_failed":
		tenantID, handleErr = h.handleInvoicePaymentFailed(ctx, event)
	case "charge.dispute.created", "charge.dispute.updated", "charge.dispute.closed":
		tenantID, handleErr = h.handleChargeDisputeEvent(ctx, event)
	default:
		handleErr = nil
	}

	if handleErr != nil {
		if updateErr := updateEventStatus(database.CloudBillingEventStatusErrored, tenantID, handleErr); updateErr != nil {
			return fmt.Errorf("mark stripe event failed: %w", updateErr)
		}
		return handleErr
	}

	if err := updateEventStatus(database.CloudBillingEventStatusDone, tenantID, nil); err != nil {
		return fmt.Errorf("mark stripe event processed: %w", err)
	}

	return nil
}

func tenantIDFromStripeMetadata(metadata map[string]string) (uuid.UUID, error) {
	tenantID, err := uuid.Parse(strings.TrimSpace(metadata["tenant_id"]))
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse subscription tenant_id: %w", err)
	}
	return tenantID, nil
}

// normalizeJurisdiction maps AWS region names or shorthand to "EU" or "US".
func normalizeJurisdiction(value string) string {
	v := strings.TrimSpace(strings.ToUpper(value))
	if strings.HasPrefix(v, "EU") {
		return "EU"
	}
	if strings.HasPrefix(v, "US") {
		return "US"
	}
	return v
}

func normalizePlanCode(planCode string) string {
	switch strings.TrimSpace(strings.ToLower(planCode)) {
	case database.CloudPlanFree:
		return database.CloudPlanFree
	case database.CloudPlanBusiness:
		return database.CloudPlanBusiness
	case database.CloudPlanPro:
		return database.CloudPlanPro
	default:
		return ""
	}
}

func planNameForCode(planCode string) string {
	switch normalizePlanCode(planCode) {
	case database.CloudPlanBusiness:
		return "Business"
	case database.CloudPlanPro:
		return "Pro"
	default:
		return "Free"
	}
}

func effectivePlanCode(account *database.CloudBillingAccount) (string, string) {
	if account == nil {
		return "", ""
	}

	switch strings.TrimSpace(account.SubscriptionStatus) {
	case "", database.CloudSubscriptionStatusFree, subscriptionStatusPending, subscriptionStatusCanceled, database.CloudSubscriptionStatusChargebackLost:
		return database.CloudPlanFree, planNameForCode(database.CloudPlanFree)
	default:
		return strings.TrimSpace(account.PlanCode), strings.TrimSpace(account.PlanName)
	}
}

func stripeCustomerName(user *api.User, teamName string) string {
	if user != nil {
		if name := strings.TrimSpace(strings.TrimSpace(user.GivenName + " " + user.LastName)); name != "" {
			return name
		}
	}
	return strings.TrimSpace(teamName)
}

func priceIDForPlan(conf *config.Config, planCode string) string {
	switch normalizePlanCode(planCode) {
	case database.CloudPlanBusiness:
		return strings.TrimSpace(conf.StripePriceBusinessMonthly)
	case database.CloudPlanPro:
		return strings.TrimSpace(conf.StripePriceProMonthly)
	default:
		return ""
	}
}

func effectiveJurisdiction(conf *config.Config, requested string) string {
	if trimmed := strings.TrimSpace(strings.ToUpper(requested)); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(strings.ToUpper(conf.CloudJurisdiction))
}

func checkoutSuccessURL(conf *config.Config) string {
	if override := strings.TrimSpace(conf.CloudCheckoutSuccessURL); override != "" {
		return override
	}
	return strings.TrimRight(conf.PublicURL, "/") + "/admin/team?checkout=success"
}

func checkoutCancelURL(conf *config.Config) string {
	if override := strings.TrimSpace(conf.CloudCheckoutCancelURL); override != "" {
		return override
	}
	return strings.TrimRight(conf.PublicURL, "/") + "/admin/team?checkout=canceled"
}

func issueLoginSession(w http.ResponseWriter, conf *config.Config, userID uuid.UUID) error {
	token, err := authcore.GenerateToken(conf.JWTSecret, conf.PublicURL, userID)
	if err != nil {
		return fmt.Errorf("generate auth token: %w", err)
	}

	isSecure := strings.HasPrefix(conf.PublicURL, "https://")
	authcore.SetTokenCookie(w, token, isSecure)
	return nil
}

func (h *handler) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) (uuid.UUID, error) {
	invoice, err := parseStripeInvoiceEvent(event.Data.Raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode invoice.payment_failed: %w", err)
	}

	account, err := h.resolveCloudBillingAccount(ctx, invoice.TenantID(), invoice.Customer.ID, invoice.Subscription.ID)
	if err != nil {
		if errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	account.SubscriptionStatus = database.CloudSubscriptionStatusPastDue
	if invoice.Subscription.ID != "" {
		account.StripeSubscriptionID = invoice.Subscription.ID
	}
	if invoice.Customer.ID != "" {
		account.StripeCustomerID = invoice.Customer.ID
	}

	return account.TenantID, h.ctx.Store.UpsertCloudBillingAccount(ctx, *account)
}

func (h *handler) handleChargeDisputeEvent(ctx context.Context, event stripe.Event) (uuid.UUID, error) {
	dispute, err := parseStripeDisputeEvent(event.Data.Raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode %s: %w", event.Type, err)
	}
	if strings.TrimSpace(dispute.Charge.ID) == "" {
		return uuid.Nil, fmt.Errorf("dispute charge id missing")
	}

	charge, err := h.stripe.GetCharge(ctx, dispute.Charge.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("load disputed charge: %w", err)
	}

	account, err := h.resolveCloudBillingAccount(ctx, uuid.Nil, charge.CustomerID, "")
	if err != nil {
		if errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	account.SubscriptionStatus = disputeSubscriptionStatus(string(event.Type), dispute.Status)
	return account.TenantID, h.ctx.Store.UpsertCloudBillingAccount(ctx, *account)
}

func (h *handler) resolveCloudBillingAccount(ctx context.Context, tenantID uuid.UUID, customerID string, subscriptionID string) (*database.CloudBillingAccount, error) {
	if tenantID != uuid.Nil {
		account, err := h.ctx.Store.GetCloudBillingAccount(ctx, tenantID)
		if err == nil {
			return account, nil
		}
		if !errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			return nil, err
		}
	}

	if strings.TrimSpace(subscriptionID) != "" {
		account, err := h.ctx.Store.GetCloudBillingAccountByStripeSubscriptionID(ctx, subscriptionID)
		if err == nil {
			return account, nil
		}
		if !errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			return nil, err
		}
	}

	if strings.TrimSpace(customerID) != "" {
		account, err := h.ctx.Store.GetCloudBillingAccountByStripeCustomerID(ctx, customerID)
		if err == nil {
			return account, nil
		}
		if !errors.Is(err, database.ErrCloudBillingAccountNotFound) {
			return nil, err
		}
	}

	return nil, database.ErrCloudBillingAccountNotFound
}

func (c *stripeSDKClient) CreateCustomer(ctx context.Context, input createCustomerInput) (string, error) {
	params := &stripe.CustomerCreateParams{
		Email: stripe.String(input.Email),
		Name:  stripe.String(strings.TrimSpace(input.Name)),
	}
	setStripeVersionHeader(&params.Params)
	setStripeIdempotencyKey(&params.Params, input.IdempotencyKey)
	params.AddMetadata("user_id", input.UserID.String())
	params.AddMetadata("tenant_id", input.TenantID.String())
	params.AddMetadata("plan_code", input.PlanCode)
	params.AddMetadata("jurisdiction", input.Jurisdiction)

	created, err := c.client.V1Customers.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	return created.ID, nil
}

func (c *stripeSDKClient) CreateCheckoutSession(ctx context.Context, input createCheckoutSessionInput) (*checkoutSessionOutput, error) {
	params := &stripe.CheckoutSessionCreateParams{
		Mode:       stripe.String(checkoutModeSubscription),
		Customer:   stripe.String(input.CustomerID),
		SuccessURL: stripe.String(input.SuccessURL),
		CancelURL:  stripe.String(input.CancelURL),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(input.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{
			Metadata: map[string]string{
				"user_id":      input.UserID.String(),
				"tenant_id":    input.TenantID.String(),
				"plan_code":    input.PlanCode,
				"plan_name":    input.PlanName,
				"jurisdiction": input.Jurisdiction,
			},
		},
		CustomerUpdate: &stripe.CheckoutSessionCreateCustomerUpdateParams{
			Name:    stripe.String("auto"),
			Address: stripe.String("auto"),
		},
	}
	setStripeVersionHeader(&params.Params)
	params.Locale = stripe.String(input.Locale)
	params.AddMetadata("user_id", input.UserID.String())
	params.AddMetadata("tenant_id", input.TenantID.String())
	params.AddMetadata("plan_code", input.PlanCode)
	params.AddMetadata("plan_name", input.PlanName)
	params.AddMetadata("jurisdiction", input.Jurisdiction)
	params.AddMetadata("email", input.Email)

	created, err := c.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create checkout session: %w", err)
	}
	return &checkoutSessionOutput{
		ID:  created.ID,
		URL: created.URL,
	}, nil
}

func (c *stripeSDKClient) CreatePortalSession(ctx context.Context, input createPortalSessionInput) (*portalSessionOutput, error) {
	params := &stripe.BillingPortalSessionCreateParams{
		Customer:  stripe.String(input.CustomerID),
		ReturnURL: stripe.String(input.ReturnURL),
	}
	setStripeVersionHeader(&params.Params)
	params.Locale = stripe.String(input.Locale)
	if strings.TrimSpace(input.ConfigurationID) != "" {
		params.Configuration = stripe.String(strings.TrimSpace(input.ConfigurationID))
	}

	created, err := c.client.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create stripe billing portal session: %w", err)
	}
	return &portalSessionOutput{
		ID:  created.ID,
		URL: created.URL,
	}, nil
}

func (c *stripeSDKClient) GetCharge(ctx context.Context, chargeID string) (*stripeChargeOutput, error) {
	params := &stripe.ChargeRetrieveParams{}
	setStripeVersionHeader(&params.Params)

	loaded, err := c.client.V1Charges.Retrieve(ctx, strings.TrimSpace(chargeID), params)
	if err != nil {
		return nil, fmt.Errorf("get stripe charge: %w", err)
	}

	return &stripeChargeOutput{
		ID:         strings.TrimSpace(loaded.ID),
		CustomerID: stripeCustomerID(loaded.Customer),
	}, nil
}

func (stripeWebhookSDK) ConstructEvent(payload []byte, header string, secret string) (stripe.Event, error) {
	event, err := webhook.ConstructEventWithOptions(payload, header, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return event, err
	}
	if strings.TrimSpace(event.APIVersion) != stripeAPIVersion {
		return event, fmt.Errorf("unexpected stripe event api version %s", strings.TrimSpace(event.APIVersion))
	}
	return event, nil
}

func billingPortalReturnURL(conf *config.Config) string {
	return strings.TrimRight(conf.PublicURL, "/") + "/admin/team"
}

func tenantIDFromStripeEvent(event stripe.Event) (uuid.UUID, error) {
	switch string(event.Type) {
	case "checkout.session.completed":
		session, err := parseStripeCheckoutSessionEvent(event.Data.Raw)
		if err != nil {
			return uuid.Nil, fmt.Errorf("decode checkout.session.completed: %w", err)
		}
		return uuid.Parse(strings.TrimSpace(session.Metadata["tenant_id"]))
	case "customer.subscription.updated", "customer.subscription.deleted":
		subscription, err := parseStripeSubscriptionEvent(event.Data.Raw)
		if err != nil {
			return uuid.Nil, fmt.Errorf("decode subscription event: %w", err)
		}
		return tenantIDFromStripeMetadata(subscription.Metadata)
	case "invoice.payment_failed":
		invoice, err := parseStripeInvoiceEvent(event.Data.Raw)
		if err != nil {
			return uuid.Nil, fmt.Errorf("decode invoice.payment_failed: %w", err)
		}
		return invoice.TenantID(), nil
	default:
		return uuid.Nil, nil
	}
}

func stripeEventProcessedAt() time.Time {
	return time.Now().UTC()
}

type stripeExpandableRef struct {
	ID string
}

func (r *stripeExpandableRef) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		r.ID = ""
		return nil
	}
	if strings.HasPrefix(trimmed, "\"") {
		return json.Unmarshal(data, &r.ID)
	}

	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	r.ID = strings.TrimSpace(obj.ID)
	return nil
}

type stripeCheckoutSessionLineItem struct {
	Price stripeExpandableRef `json:"price"`
}

type stripeCheckoutSessionEventPayload struct {
	Metadata     map[string]string               `json:"metadata"`
	Status       string                          `json:"status"`
	Customer     stripeExpandableRef             `json:"customer"`
	Subscription stripeExpandableRef             `json:"subscription"`
	LineItems    []stripeCheckoutSessionLineItem `json:"line_items"`
}

func (p *stripeCheckoutSessionEventPayload) FirstPriceID() string {
	if p == nil || len(p.LineItems) == 0 {
		return ""
	}
	return strings.TrimSpace(p.LineItems[0].Price.ID)
}

type stripeSubscriptionItem struct {
	Price stripeExpandableRef `json:"price"`
}

type stripeSubscriptionItems struct {
	Data []stripeSubscriptionItem `json:"data"`
}

type stripeSubscriptionEventPayload struct {
	ID       string                  `json:"id"`
	Metadata map[string]string       `json:"metadata"`
	Status   string                  `json:"status"`
	Customer stripeExpandableRef     `json:"customer"`
	Items    stripeSubscriptionItems `json:"items"`
}

func (p *stripeSubscriptionEventPayload) FirstPriceID() string {
	if p == nil || len(p.Items.Data) == 0 {
		return ""
	}
	return strings.TrimSpace(p.Items.Data[0].Price.ID)
}

type stripeInvoiceEventPayload struct {
	Customer     stripeExpandableRef `json:"customer"`
	Subscription stripeExpandableRef `json:"subscription"`
	Metadata     map[string]string   `json:"metadata"`
}

func (p *stripeInvoiceEventPayload) TenantID() uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	tenantID, err := tenantIDFromStripeMetadata(p.Metadata)
	if err != nil {
		return uuid.Nil
	}
	return tenantID
}

type stripeDisputeEventPayload struct {
	Charge stripeExpandableRef `json:"charge"`
	Status string              `json:"status"`
}

func parseStripeInvoiceEvent(raw []byte) (*stripeInvoiceEventPayload, error) {
	var invoice stripeInvoiceEventPayload
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return nil, err
	}
	return &invoice, nil
}

func parseStripeCheckoutSessionEvent(raw []byte) (*stripeCheckoutSessionEventPayload, error) {
	var session stripeCheckoutSessionEventPayload
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func parseStripeSubscriptionEvent(raw []byte) (*stripeSubscriptionEventPayload, error) {
	var subscription stripeSubscriptionEventPayload
	if err := json.Unmarshal(raw, &subscription); err != nil {
		return nil, err
	}
	return &subscription, nil
}

func parseStripeDisputeEvent(raw []byte) (*stripeDisputeEventPayload, error) {
	var dispute stripeDisputeEventPayload
	if err := json.Unmarshal(raw, &dispute); err != nil {
		return nil, err
	}
	return &dispute, nil
}

func disputeSubscriptionStatus(eventType string, disputeStatus string) string {
	normalizedStatus := strings.TrimSpace(strings.ToLower(disputeStatus))
	if eventType == "charge.dispute.closed" {
		switch normalizedStatus {
		case "won":
			return database.CloudSubscriptionStatusActive
		case "lost":
			return database.CloudSubscriptionStatusChargebackLost
		}
	}
	return database.CloudSubscriptionStatusDisputed
}

func normalizeStripeLocale(raw string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "_", "-"))
	if trimmed == "" {
		return "auto"
	}

	parts := strings.Split(trimmed, "-")
	normalized := make([]string, 0, len(parts))
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "auto"
		}
		if index == 0 {
			normalized = append(normalized, strings.ToLower(part))
			continue
		}
		if len(part) == 2 {
			normalized = append(normalized, strings.ToUpper(part))
			continue
		}
		normalized = append(normalized, strings.ToLower(part))
	}
	tag := strings.Join(normalized, "-")

	if supported, ok := supportedStripeLocale(tag); ok {
		return supported
	}
	if base, _, found := strings.Cut(tag, "-"); found {
		if supported, ok := supportedStripeLocale(base); ok {
			return supported
		}
	}
	return "auto"
}

func stripeCustomerID(customer *stripe.Customer) string {
	if customer == nil {
		return ""
	}
	return strings.TrimSpace(customer.ID)
}

func newStripeSDKClient(secretKey string) *stripeSDKClient {
	return &stripeSDKClient{
		client: stripe.NewClient(strings.TrimSpace(secretKey)),
	}
}

func setStripeVersionHeader(params *stripe.Params) {
	if params == nil {
		return
	}
	if params.Headers == nil {
		params.Headers = make(http.Header)
	}
	params.Headers.Set("Stripe-Version", stripeAPIVersion)
}

func setStripeIdempotencyKey(params *stripe.Params, key string) {
	if params == nil {
		return
	}
	if trimmed := strings.TrimSpace(key); trimmed != "" {
		params.SetIdempotencyKey(trimmed)
	}
}

func stripeCustomerCreateIdempotencyKey(userID uuid.UUID, tenantID uuid.UUID, email string) string {
	return stripeIdempotencyKey("customer-create", tenantID.String(), userID.String(), strings.ToLower(strings.TrimSpace(email)))
}

func stripeIdempotencyKey(operation string, parts ...string) string {
	normalized := make([]string, 0, len(parts)+2)
	normalized = append(normalized, "hitkeep", strings.TrimSpace(operation))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return strings.Join(normalized, ":")
}

func supportedStripeLocale(tag string) (string, bool) {
	switch tag {
	case "auto", "bg", "cs", "da", "de", "el", "en", "en-GB", "es", "es-419", "et", "fi", "fil", "fr", "fr-CA", "hr", "hu", "id", "it", "ja", "ko", "lt", "lv", "ms", "mt", "nb", "nl", "pl", "pt", "pt-BR", "ro", "ru", "sk", "sl", "sv", "th", "tr", "vi", "zh", "zh-HK", "zh-TW":
		return tag, true
	default:
		return "", false
	}
}

func jurisdictionFromStripeEvent(event stripe.Event) (string, error) {
	switch string(event.Type) {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return "", fmt.Errorf("decode checkout.session.completed: %w", err)
		}
		return strings.TrimSpace(strings.ToUpper(session.Metadata["jurisdiction"])), nil
	case "customer.subscription.updated", "customer.subscription.deleted":
		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			return "", fmt.Errorf("decode subscription event: %w", err)
		}
		return strings.TrimSpace(strings.ToUpper(subscription.Metadata["jurisdiction"])), nil
	case "invoice.payment_failed":
		invoice, err := parseStripeInvoiceEvent(event.Data.Raw)
		if err != nil {
			return "", fmt.Errorf("decode invoice.payment_failed: %w", err)
		}
		return strings.TrimSpace(strings.ToUpper(invoice.Metadata["jurisdiction"])), nil
	default:
		return "", nil
	}
}
