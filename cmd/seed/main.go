// seed populates a HitKeep database with realistic demo data so the dashboard
// looks great for screenshots and product demos.
//
// Usage:
//
//	go run ./cmd/seed -db hitkeep.db -email demo@example.com -password demo1234
//
// The script:
//   - Creates (or reuses) a demo user.
//   - Creates a demo site "acme-analytics.io".
//   - Inserts ~12 000 pageviews over the past N days with realistic country,
//     device, referrer, and UTM distributions.
//   - Creates conversion events: newsletter_signup, trial_started,
//     demo_requested, purchase_completed.
//   - Creates GA4-inspired ecommerce events: view_item, add_to_cart,
//     begin_checkout, purchase.
//   - Sets up Goals and Funnels that reference those events.
//   - Runs the rollup backfill so all charts are populated immediately.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"hitkeep/internal/api"
	"hitkeep/internal/auth"
	"hitkeep/internal/database"
	"hitkeep/internal/worker"
)

// hashPassword produces an argon2id hash in the same format the server expects.
// Must match auth.HashPassword in internal/server/auth/handlers.go.
func hashPassword(password string) (string, error) {
	const (
		timeCost = 1
		memory   = 64 * 1024
		threads  = 4
		keyLen   = 32
		saltLen  = 16
	)
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, keyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, timeCost, threads, b64Salt, b64Hash), nil
}

// ─────────────────────────────────────────────
// Weighted random helpers
// ─────────────────────────────────────────────

type weightedEntry[T any] struct {
	value  T
	weight int
}

func pickWeighted[T any](rng *mrand.Rand, entries []weightedEntry[T]) T {
	total := 0
	for _, e := range entries {
		total += e.weight
	}
	n := rng.Intn(total)
	for _, e := range entries {
		n -= e.weight
		if n < 0 {
			return e.value
		}
	}
	return entries[len(entries)-1].value
}

// ─────────────────────────────────────────────
// Data tables
// ─────────────────────────────────────────────

var pages = []weightedEntry[string]{
	{"/", 280},
	{"/pricing", 130},
	{"/features", 90},
	{"/blog/privacy-first-analytics-2025", 80},
	{"/blog/self-hosted-vs-cloud-analytics", 70},
	{"/blog/replace-google-analytics", 65},
	{"/blog/gdpr-compliant-analytics", 60},
	{"/docs/getting-started", 70},
	{"/docs/configuration", 50},
	{"/docs/api-reference", 40},
	{"/about", 50},
	{"/changelog", 40},
	{"/signup", 110},
	{"/login", 55},
	{"/contact", 40},
	{"/terms", 20},
}

var referrers = []weightedEntry[*string]{
	{nil, 380},                                       // direct / typed
	{new("https://www.google.com"), 200},             // Google search
	{new("https://news.ycombinator.com"), 120},       // HN
	{new("https://twitter.com"), 80},                 // Twitter/X
	{new("https://www.reddit.com/r/selfhosted"), 60}, // Reddit
	{new("https://www.linkedin.com"), 50},            // LinkedIn
	{new("https://github.com/hitkeep/hitkeep"), 50},  // GitHub
	{new("https://dev.to"), 40},                      // dev.to
	{new("https://lobste.rs"), 30},                   // Lobsters
	{new("https://www.producthunt.com"), 20},         // Product Hunt
}

var countries = []weightedEntry[*string]{
	{new("US"), 350},
	{new("DE"), 100},
	{new("GB"), 80},
	{new("CA"), 60},
	{new("FR"), 50},
	{new("NL"), 50},
	{new("AU"), 40},
	{new("SE"), 30},
	{new("CH"), 30},
	{new("IN"), 35},
	{new("JP"), 25},
	{new("BR"), 25},
	{new("PL"), 20},
	{new("ES"), 20},
	{new("IT"), 20},
	{new("NO"), 15},
	{new("FI"), 15},
	{new("DK"), 15},
	{new("SG"), 15},
}

type viewportPreset struct {
	vw, vh, sw, sh int
}

var desktopViewports = []viewportPreset{
	{1920, 1080, 1920, 1080},
	{1440, 900, 1440, 900},
	{1440, 810, 2560, 1440},
	{1280, 800, 1280, 800},
	{1366, 768, 1366, 768},
	{1536, 864, 1536, 864},
}

var mobileViewports = []viewportPreset{
	{390, 844, 390, 844}, // iPhone 14
	{430, 932, 430, 932}, // iPhone 14 Pro Max
	{412, 892, 412, 892}, // Pixel 7
	{360, 800, 360, 800}, // Android mid-range
}

var tabletViewports = []viewportPreset{
	{768, 1024, 768, 1024},
	{810, 1080, 810, 1080},
}

type uaGroup struct {
	ua   string
	kind string // "desktop", "mobile", "tablet"
}

var userAgents = []weightedEntry[uaGroup]{
	{uaGroup{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", "desktop"}, 280},
	{uaGroup{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", "desktop"}, 230},
	{uaGroup{"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15", "desktop"}, 90},
	{uaGroup{"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0", "desktop"}, 80},
	{uaGroup{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0", "desktop"}, 40},
	{uaGroup{"Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36", "mobile"}, 110},
	{uaGroup{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1", "mobile"}, 100},
	{uaGroup{"Mozilla/5.0 (Linux; Android 12; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36", "mobile"}, 50},
	{uaGroup{"Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1", "tablet"}, 60},
	{uaGroup{"Mozilla/5.0 (Linux; Android 11; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", "tablet"}, 40},
}

var languages = []weightedEntry[*string]{
	{new("en-US"), 380},
	{new("en-GB"), 80},
	{new("de-DE"), 100},
	{new("fr-FR"), 50},
	{new("nl-NL"), 50},
	{new("sv-SE"), 30},
	{new("pt-BR"), 25},
	{new("ja-JP"), 25},
	{new("es-ES"), 20},
	{new("pl-PL"), 20},
	{new("it-IT"), 20},
}

type utmParams struct {
	source, medium, campaign string
	term, content            *string
}

type ecommerceProduct struct {
	itemID    string
	itemName  string
	plan      string
	category  string
	price     int
	priceYear int
}

var utmCampaigns = []weightedEntry[*utmParams]{
	{nil, 800}, // no UTM (organic / direct)
	{&utmParams{
		source: "twitter", medium: "social", campaign: "product-launch-2025",
	}, 40},
	{&utmParams{
		source: "google", medium: "cpc", campaign: "self-hosted-analytics",
		term: new("self hosted analytics"), content: new("headline-a"),
	}, 35},
	{&utmParams{
		source: "google", medium: "cpc", campaign: "self-hosted-analytics",
		term: new("open source analytics"), content: new("headline-b"),
	}, 25},
	{&utmParams{
		source: "newsletter", medium: "email", campaign: "monthly-digest-jan",
	}, 30},
	{&utmParams{
		source: "newsletter", medium: "email", campaign: "monthly-digest-feb",
	}, 30},
	{&utmParams{
		source: "reddit", medium: "social", campaign: "selfhosted-post",
	}, 20},
	{&utmParams{
		source: "linkedin", medium: "social", campaign: "b2b-outreach",
	}, 20},
	{&utmParams{
		source: "producthunt", medium: "referral", campaign: "ph-launch-day",
	}, 15},
	{&utmParams{
		source: "devto", medium: "content", campaign: "tutorial-series",
	}, 15},
}

var ecommerceProducts = []weightedEntry[ecommerceProduct]{
	{ecommerceProduct{itemID: "starter-plan", itemName: "Starter Plan", plan: "starter", category: "subscription", price: 29, priceYear: 290}, 34},
	{ecommerceProduct{itemID: "pro-plan", itemName: "Pro Plan", plan: "pro", category: "subscription", price: 79, priceYear: 790}, 36},
	{ecommerceProduct{itemID: "business-plan", itemName: "Business Plan", plan: "business", category: "subscription", price: 199, priceYear: 1990}, 20},
	{ecommerceProduct{itemID: "team-seat-pack", itemName: "Team Seat Pack", plan: "business", category: "add-on", price: 49, priceYear: 490}, 7},
	{ecommerceProduct{itemID: "annual-upgrade", itemName: "Annual Upgrade", plan: "pro", category: "upgrade", price: 199, priceYear: 199}, 3},
}

// ─────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────

func main() {
	dbPath := flag.String("db", "hitkeep.db", "Path to hitkeep.db")
	dataPath := flag.String("data-path", "", "Base directory for per-tenant data files (default: directory containing -db)")
	email := flag.String("email", "demo@example.com", "Demo user email")
	password := flag.String("password", "demo1234", "Demo user password")
	days := flag.Int("days", 90, "Days of demo traffic to generate")
	domain := flag.String("domain", "acme-analytics.io", "Demo site domain")
	seed := flag.Int64("seed", 42, "Random seed for reproducibility")
	shareToken := flag.String("share-token", "", "Create a share link with this exact token (64-char hex string)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx := context.Background()

	store := database.NewStore(*dbPath)
	if err := store.Connect(); err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		slog.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}
	tenantBasePath := strings.TrimSpace(*dataPath)
	if tenantBasePath == "" {
		tenantBasePath = filepath.Dir(*dbPath)
	}

	tenantMgr := database.NewTenantStoreManager(store, tenantBasePath)
	defer tenantMgr.Close()

	rng := mrand.New(mrand.NewSource(*seed)) // #nosec G404 — demo data seeding; reproducibility trumps entropy

	// Seed extra users first so that the demo site (created below) is the
	// most-recently-created entry. GetSites orders by created_at DESC, so the
	// first item in the list — which the UI auto-selects — will be the demo
	// site that actually has traffic data.
	seedAdditionalUsers(ctx, store)

	slog.Info("Creating demo user", "email", *email)
	userID := ensureUser(ctx, store, *email, *password)

	// Ensure the demo team exists and is active BEFORE creating the site,
	// so the site is assigned to the intended tenant.
	seedTeam(ctx, store, userID)

	slog.Info("Creating demo site", "domain", *domain)
	site, err := ensureSiteInActiveTeam(ctx, store, userID, *domain)
	if err != nil {
		slog.Error("Failed to ensure demo site", "error", err)
		os.Exit(1)
	}
	siteID := site.ID
	slog.Info("Site created", "site_id", siteID)

	siteTenantID, err := store.GetSiteTenantID(ctx, siteID)
	if err != nil {
		slog.Error("Failed to resolve site tenant", "site_id", siteID, "error", err)
		os.Exit(1)
	}
	seedAPIClients(ctx, store, userID, siteTenantID, siteID)
	if *shareToken != "" {
		seedShareLink(ctx, store, siteID, userID, *shareToken)
	}
	if err := tenantMgr.SyncSite(ctx, siteID); err != nil {
		slog.Error("Failed to sync tenant site metadata", "site_id", siteID, "tenant_id", siteTenantID, "error", err)
		os.Exit(1)
	}
	analyticsStore, err := tenantMgr.ForTenant(ctx, siteTenantID)
	if err != nil {
		slog.Error("Failed to resolve tenant analytics store", "tenant_id", siteTenantID, "error", err)
		os.Exit(1)
	}
	slog.Info("Resolved tenant analytics store", "tenant_id", siteTenantID)

	goalIDs := createGoals(ctx, analyticsStore, siteID)
	createFunnels(ctx, analyticsStore, siteID)

	slog.Info("Seeding traffic", "days", *days)
	stats := seedTraffic(ctx, analyticsStore, siteID, goalIDs, *days, rng)

	slog.Info("Running rollup backfill...")
	rollupWorker := worker.NewRollupBackfillWorker(tenantMgr)
	if err := rollupWorker.Run(ctx); err != nil {
		slog.Error("Rollup backfill failed — charts may be incomplete", "error", err)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║       Demo data seeded successfully!         ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("  Email:         %s\n", *email)
	fmt.Printf("  Password:      %s\n", *password)
	fmt.Printf("  Site:          %s (%s)\n", *domain, siteID)
	fmt.Printf("  Tenant Data:   %s\n", tenantBasePath)
	fmt.Printf("  Pageviews:     %d\n", stats.hits)
	fmt.Printf("  Sessions:      %d\n", stats.sessions)
	fmt.Printf("  Events:        %d\n", stats.events)
	fmt.Printf("  Period:        last %d days\n", *days)
	fmt.Println()
}

// ─────────────────────────────────────────────
// User
// ─────────────────────────────────────────────

func ensureUser(ctx context.Context, store *database.Store, email, password string) uuid.UUID {
	existing, err := store.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Error("Failed to look up user", "error", err)
		os.Exit(1)
	}
	if existing != nil {
		slog.Info("Reusing existing user", "id", existing.ID)
		return existing.ID
	}

	hash, err := hashPassword(password)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		os.Exit(1)
	}

	id, err := store.CreateUser(ctx, email, hash)
	if err != nil {
		slog.Error("Failed to create demo user", "error", err)
		os.Exit(1)
	}
	return id
}

func seedAPIClients(ctx context.Context, store *database.Store, userID, tenantID, siteID uuid.UUID) {
	personalName := "Personal Export Token"
	personalClients, err := store.ListAPIClients(ctx, userID)
	if err != nil {
		slog.Warn("Failed to list personal API clients", "error", err)
	} else if !hasAPIClientNamed(personalClients, personalName) {
		_, _, err := store.CreateAPIClient(ctx, userID, personalName, "Read-only export token for personal automation", auth.InstanceUser, map[uuid.UUID]auth.SiteRole{
			siteID: auth.SiteViewer,
		}, nil)
		if err != nil {
			slog.Warn("Failed to seed personal API client", "error", err)
		}
	}

	teamName := "Shared Team Integration"
	teamClients, err := store.ListTeamAPIClients(ctx, tenantID)
	if err != nil {
		slog.Warn("Failed to list team API clients", "tenant_id", tenantID, "error", err)
	} else if !hasAPIClientNamed(teamClients, teamName) {
		_, _, err := store.CreateTeamAPIClient(ctx, tenantID, teamName, "Team-owned token for shared dashboards and CI exports", map[uuid.UUID]auth.SiteRole{
			siteID: auth.SiteAdmin,
		}, nil)
		if err != nil {
			slog.Warn("Failed to seed team API client", "tenant_id", tenantID, "error", err)
		}
	}

	slog.Info("Demo API clients ensured", "user_id", userID, "tenant_id", tenantID)
}

// seedShareLink creates a share link with a predetermined token so the public
// URL stays stable across database resets.
func seedShareLink(ctx context.Context, store *database.Store, siteID, createdBy uuid.UUID, token string) {
	if len(token) != 64 {
		slog.Error("Share token must be a 64-character hex string", "len", len(token))
		os.Exit(1)
	}
	if _, err := hex.DecodeString(token); err != nil {
		slog.Error("Share token is not valid hex", "error", err)
		os.Exit(1)
	}

	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	// Check if a share link with this token already exists.
	var exists bool
	_ = store.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) > 0 FROM share_links WHERE token_hash = ? AND revoked_at IS NULL",
		tokenHash,
	).Scan(&exists)
	if exists {
		slog.Info("Share link already exists, skipping", "token_hint", tokenHash[:8])
		return
	}

	linkID := uuid.New()
	now := time.Now().UTC()
	if err := store.Exec(ctx,
		"INSERT INTO share_links (id, site_id, token_hash, created_by, created_at) VALUES (?, ?, ?, ?, ?)",
		linkID, siteID, tokenHash, createdBy, now,
	); err != nil {
		slog.Error("Failed to create share link", "error", err)
		os.Exit(1)
	}
	slog.Info("Share link seeded", "token_hint", tokenHash[:8])
}

// ─────────────────────────────────────────────
// Goals & Funnels
// ─────────────────────────────────────────────

type goalIDs struct {
	newsletter uuid.UUID
	trial      uuid.UUID
	purchase   uuid.UUID
	demo       uuid.UUID
	pricing    uuid.UUID
}

func createGoals(ctx context.Context, store *database.Store, siteID uuid.UUID) goalIDs {
	create := func(name, typ, value string) uuid.UUID {
		g := &api.Goal{SiteID: siteID, Name: name, Type: typ, Value: value}
		if err := store.CreateGoal(ctx, g); err != nil {
			slog.Error("Failed to create goal", "name", name, "error", err)
			os.Exit(1)
		}
		// Fetch it back to get the assigned ID.
		goals, err := store.GetGoals(ctx, siteID)
		if err != nil || len(goals) == 0 {
			slog.Error("Failed to fetch goals after creation", "error", err)
			os.Exit(1)
		}
		for _, goal := range goals {
			if goal.Name == name {
				return goal.ID
			}
		}
		return uuid.Nil
	}

	ids := goalIDs{
		newsletter: create("Newsletter Signup", "event", "newsletter_signup"),
		trial:      create("Free Trial Started", "event", "trial_started"),
		purchase:   create("Purchase Completed", "event", "purchase_completed"),
		demo:       create("Demo Requested", "event", "demo_requested"),
		pricing:    create("Pricing Page", "path", "/pricing"),
	}

	slog.Info("Goals created", "count", 5)
	return ids
}

func createFunnels(ctx context.Context, store *database.Store, siteID uuid.UUID) {
	funnels := []api.Funnel{
		{
			SiteID: siteID,
			Name:   "Acquisition Funnel",
			Steps: []api.FunnelStep{
				{Type: "path", Value: "/"},
				{Type: "path", Value: "/pricing"},
				{Type: "path", Value: "/signup"},
				{Type: "event", Value: "trial_started"},
			},
		},
		{
			SiteID: siteID,
			Name:   "Purchase Funnel",
			Steps: []api.FunnelStep{
				{Type: "path", Value: "/pricing"},
				{Type: "path", Value: "/signup"},
				{Type: "event", Value: "purchase_completed"},
			},
		},
	}

	for _, f := range funnels {
		if err := store.CreateFunnel(ctx, &f); err != nil {
			slog.Error("Failed to create funnel", "name", f.Name, "error", err)
			os.Exit(1)
		}
	}
	slog.Info("Funnels created", "count", len(funnels))
}

// ─────────────────────────────────────────────
// Traffic seeding
// ─────────────────────────────────────────────

type seedStats struct {
	hits     int
	sessions int
	events   int
}

func seedTraffic(ctx context.Context, store *database.Store, siteID uuid.UUID, goals goalIDs, numDays int, rng *mrand.Rand) seedStats {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -numDays).Truncate(24 * time.Hour)

	var stats seedStats

	for d := range numDays {
		day := start.Add(time.Duration(d) * 24 * time.Hour)
		weekday := day.Weekday()

		// Base traffic: higher on weekdays, lower on weekends.
		base := 180
		if weekday == time.Saturday || weekday == time.Sunday {
			base = 80
		}

		// Growth trend: traffic grows ~80% over the period.
		growth := 1.0 + (float64(d)/float64(numDays))*0.8
		// Random daily variation ±25%.
		variation := 0.75 + rng.Float64()*0.5

		// Occasional spikes (blog post, HN post, etc.) — roughly once every 2 weeks.
		if rng.Float64() < 0.07 {
			variation *= 2.5 + rng.Float64()*2.0
		}

		dailyHits := max(int(float64(base)*growth*variation), 10)

		// Distribute hits across the day in sessions of 1-5 pages.
		hitsLeft := dailyHits
		for hitsLeft > 0 {
			sessionLen := min(1+rng.Intn(5), hitsLeft)
			hitsLeft -= sessionLen

			sessionID := uuid.New()
			stats.sessions++

			// Pick session-level attributes that stay constant within a session.
			uaEntry := pickWeighted(rng, userAgents)
			country := pickWeighted(rng, countries)
			lang := pickWeighted(rng, languages)
			utmEntry := pickWeighted(rng, utmCampaigns)
			ref := pickWeighted(rng, referrers)

			vw, vh, sw, sh := pickViewport(rng, uaEntry.kind)

			// First page of the session is often the entry page.
			entryPage := pickWeighted(rng, pages)

			sessionStart := randomTimeInDay(rng, day)

			for i := range sessionLen {
				var page string
				if i == 0 {
					page = entryPage
				} else {
					// Subsequent pages tend to stay in related sections.
					page = pickWeighted(rng, pages)
				}

				ts := sessionStart.Add(time.Duration(i*90+rng.Intn(120)) * time.Second)

				h := &api.Hit{
					SiteID:         siteID,
					SessionID:      sessionID,
					PageID:         uuid.New(),
					Timestamp:      ts,
					Path:           page,
					UserAgent:      new(uaEntry.ua),
					CountryCode:    country,
					Language:       lang,
					ViewportWidth:  new(vw),
					ViewportHeight: new(vh),
					ScreenWidth:    new(sw),
					ScreenHeight:   new(sh),
					IsUnique:       new(i == 0), // only the first hit in a session is unique
				}

				// Referrer only on the first hit of a session.
				if i == 0 {
					h.Referrer = ref
				}

				// UTM params only on the first hit.
				if i == 0 && utmEntry != nil {
					h.UTMSource = new(utmEntry.source)
					h.UTMMedium = new(utmEntry.medium)
					h.UTMCampaign = new(utmEntry.campaign)
					h.UTMTerm = utmEntry.term
					h.UTMContent = utmEntry.content
				}

				if err := store.CreateHit(ctx, h); err != nil {
					slog.Error("Failed to insert hit", "error", err)
					continue
				}
				stats.hits++
			}

			// Maybe fire a conversion event at the end of the session.
			events := fireConversionEvents(ctx, store, siteID, sessionID, goals, rng, sessionStart.Add(time.Duration(sessionLen*90+30)*time.Second), entryPage, utmEntry)
			stats.events += events
		}

		if d%10 == 0 || d == numDays-1 {
			slog.Info("Progress", "day", d+1, "of", numDays, "hits_so_far", stats.hits)
		}
	}

	return stats
}

// fireConversionEvents randomly fires zero or more conversion events for a session.
func fireConversionEvents(ctx context.Context, store *database.Store, siteID, sessionID uuid.UUID, goals goalIDs, rng *mrand.Rand, ts time.Time, entryPage string, utm *utmParams) int {
	_ = goals // goalIDs are used for reference, events use the same string values
	count := 0

	type conversionEvent struct {
		prob  float64
		name  string
		props map[string]any
	}

	conversions := []conversionEvent{
		{0.030, "newsletter_signup", map[string]any{
			"source": randomSignupSource(rng),
			"format": randomNewsletterFormat(rng),
		}},
		{0.020, "trial_started", map[string]any{
			"plan":    randomTrialPlan(rng),
			"billing": randomBilling(rng),
		}},
		{0.015, "demo_requested", map[string]any{
			"company_size": randomCompanySize(rng),
			"industry":     randomIndustry(rng),
			"source":       randomDemoSource(rng),
		}},
	}

	for _, c := range conversions {
		if rng.Float64() < c.prob {
			ev := &api.Event{
				SiteID:     siteID,
				SessionID:  sessionID,
				Name:       c.name,
				Properties: c.props,
				Timestamp:  ts,
			}
			if err := store.CreateEvent(ctx, ev); err != nil {
				slog.Error("Failed to insert event", "name", c.name, "error", err)
				continue
			}
			count++
		}
	}
	count += fireEcommerceEvents(ctx, store, siteID, sessionID, rng, ts, entryPage, utm)
	return count
}

func fireEcommerceEvents(ctx context.Context, store *database.Store, siteID, sessionID uuid.UUID, rng *mrand.Rand, ts time.Time, entryPage string, utm *utmParams) int {
	product := pickWeighted(rng, ecommerceProducts)
	viewProb := 0.16
	cartProb := 0.42
	checkoutProb := 0.58
	purchaseProb := 0.62

	switch entryPage {
	case "/pricing", "/signup":
		viewProb = 0.42
		cartProb = 0.56
		checkoutProb = 0.66
		purchaseProb = 0.7
	case "/features", "/docs/getting-started":
		viewProb = 0.26
	}

	if utm != nil {
		switch strings.ToLower(strings.TrimSpace(utm.source)) {
		case "google", "newsletter", "producthunt":
			viewProb += 0.07
			cartProb += 0.05
			checkoutProb += 0.04
			purchaseProb += 0.05
		case "linkedin":
			viewProb += 0.04
			checkoutProb += 0.03
		}
	}

	if rng.Float64() >= minFloat(viewProb, 0.88) {
		return 0
	}

	count := 0
	billing := randomBilling(rng)
	coupon := randomCoupon(rng)
	items, totalValue, totalQuantity, primary := randomPurchaseItems(rng, product, billing)
	currency := "USD"

	viewProps := buildCatalogEventProps(items[0], currency)
	if insertSeedEvent(ctx, store, &api.Event{
		SiteID:     siteID,
		SessionID:  sessionID,
		Name:       "view_item",
		Properties: viewProps,
		Timestamp:  ts,
	}) {
		count++
	}

	if rng.Float64() >= minFloat(cartProb, 0.92) {
		return count
	}

	cartProps := buildCatalogEventProps(items[0], currency)
	cartProps["quantity"] = items[0]["quantity"]
	cartProps["items"] = items
	if insertSeedEvent(ctx, store, &api.Event{
		SiteID:     siteID,
		SessionID:  sessionID,
		Name:       "add_to_cart",
		Properties: cartProps,
		Timestamp:  ts.Add(45 * time.Second),
	}) {
		count++
	}

	if rng.Float64() >= minFloat(checkoutProb, 0.94) {
		return count
	}

	checkoutProps := map[string]any{
		"checkout_id": fmt.Sprintf("chk_%s", uuid.NewString()[:12]),
		"value":       totalValue,
		"currency":    currency,
		"items_count": totalQuantity,
		"coupon":      coupon,
		"items":       items,
	}
	if insertSeedEvent(ctx, store, &api.Event{
		SiteID:     siteID,
		SessionID:  sessionID,
		Name:       "begin_checkout",
		Properties: checkoutProps,
		Timestamp:  ts.Add(90 * time.Second),
	}) {
		count++
	}

	if rng.Float64() >= minFloat(purchaseProb, 0.96) {
		return count
	}

	transactionID := fmt.Sprintf("ord_%s", uuid.NewString()[:12])
	purchaseProps := map[string]any{
		"transaction_id": transactionID,
		"order_id":       transactionID,
		"value":          totalValue,
		"amount":         totalValue,
		"currency":       currency,
		"items_count":    totalQuantity,
		"billing":        billing,
		"tax":            0,
		"shipping":       0,
		"coupon":         coupon,
		"items":          items,
	}
	if insertSeedEvent(ctx, store, &api.Event{
		SiteID:     siteID,
		SessionID:  sessionID,
		Name:       "purchase",
		Properties: purchaseProps,
		Timestamp:  ts.Add(3 * time.Minute),
	}) {
		count++
	}

	legacyPurchaseProps := map[string]any{
		"plan":     primary.plan,
		"billing":  billing,
		"amount":   totalValue,
		"currency": currency,
	}
	if coupon != "" {
		legacyPurchaseProps["coupon"] = coupon
	}
	if insertSeedEvent(ctx, store, &api.Event{
		SiteID:     siteID,
		SessionID:  sessionID,
		Name:       "purchase_completed",
		Properties: legacyPurchaseProps,
		Timestamp:  ts.Add(3*time.Minute + 10*time.Second),
	}) {
		count++
	}

	return count
}

func insertSeedEvent(ctx context.Context, store *database.Store, event *api.Event) bool {
	if err := store.CreateEvent(ctx, event); err != nil {
		slog.Error("Failed to insert event", "name", event.Name, "error", err)
		return false
	}
	return true
}

func buildCatalogEventProps(item map[string]any, currency string) map[string]any {
	props := map[string]any{
		"item_id":   item["item_id"],
		"item_name": item["item_name"],
		"category":  item["item_category"],
		"price":     item["price"],
		"currency":  currency,
		"items":     []map[string]any{item},
	}
	if quantity, ok := item["quantity"]; ok {
		props["quantity"] = quantity
	}
	return props
}

func randomPurchaseItems(rng *mrand.Rand, product ecommerceProduct, billing string) ([]map[string]any, float64, int, ecommerceProduct) {
	primaryPrice := productPrice(product, billing)
	primaryQuantity := 1
	if product.category == "add-on" {
		primaryQuantity = 1 + rng.Intn(3)
	}

	items := []map[string]any{
		{
			"item_id":       product.itemID,
			"item_name":     product.itemName,
			"item_category": product.category,
			"price":         primaryPrice,
			"quantity":      primaryQuantity,
		},
	}
	total := float64(primaryPrice * primaryQuantity)
	totalQuantity := primaryQuantity

	if product.plan == "business" && rng.Float64() < 0.42 {
		seatPack := ecommerceProduct{itemID: "team-seat-pack", itemName: "Team Seat Pack", plan: "business", category: "add-on", price: 49, priceYear: 490}
		quantity := 1 + rng.Intn(3)
		items = append(items, map[string]any{
			"item_id":       seatPack.itemID,
			"item_name":     seatPack.itemName,
			"item_category": seatPack.category,
			"price":         seatPack.price,
			"quantity":      quantity,
		})
		total += float64(seatPack.price * quantity)
		totalQuantity += quantity
	}

	if billing == "annual" && rng.Float64() < 0.18 {
		upgrade := ecommerceProduct{itemID: "annual-upgrade", itemName: "Annual Upgrade", plan: product.plan, category: "upgrade", price: 199, priceYear: 199}
		items = append(items, map[string]any{
			"item_id":       upgrade.itemID,
			"item_name":     upgrade.itemName,
			"item_category": upgrade.category,
			"price":         upgrade.price,
			"quantity":      1,
		})
		total += float64(upgrade.price)
		totalQuantity++
	}

	return items, total, totalQuantity, product
}

// ─────────────────────────────────────────────
// Additional users / sites (admin panel population)
// ─────────────────────────────────────────────

// seedAdditionalUsers creates extra users and their sites so that the admin
// panel looks populated in screenshots. No traffic is generated for them.
func seedAdditionalUsers(ctx context.Context, store *database.Store) {
	extra := []struct{ email, domain string }{
		{"bob@devtools.co", "devtools.co"},
		{"diana@saaslaunch.com", "saaslaunch.com"},
	}

	created := 0
	for _, u := range extra {
		hash, err := hashPassword("demoPwd!1")
		if err != nil {
			continue
		}
		id, err := store.CreateUser(ctx, u.email, hash)
		if err != nil {
			continue // already exists — skip
		}
		_, _ = store.CreateSite(ctx, id, u.domain)
		created++
	}
	slog.Info("Additional users seeded for admin panel", "count", created)
}

// seedTeam creates a realistic team ("Acme Analytics") so the team settings
// and team invite email previews look great out of the box. It also sets the
// team as the demo user's active tenant so that subsequent CreateSite calls
// assign sites to this team.
func seedTeam(ctx context.Context, store *database.Store, ownerID uuid.UUID) {
	teamName := "Acme Analytics"
	teamLogo := ""
	var team *api.Team

	teams, _, err := store.ListUserTeams(ctx, ownerID)
	if err == nil {
		for _, candidate := range teams {
			if strings.EqualFold(strings.TrimSpace(candidate.Name), teamName) {
				c := candidate
				team = &c
				break
			}
		}
	}

	if team == nil {
		created, createErr := store.CreateTenant(ctx, ownerID, teamName, teamLogo)
		if createErr != nil {
			slog.Warn("Failed to create demo team; retrying lookup", "error", createErr)
			teams, _, err = store.ListUserTeams(ctx, ownerID)
			if err == nil {
				for _, candidate := range teams {
					if strings.EqualFold(strings.TrimSpace(candidate.Name), teamName) {
						c := candidate
						team = &c
						break
					}
				}
			}
			if team == nil {
				slog.Warn("Failed to resolve demo team", "error", createErr)
				return
			}
		} else {
			team = created
		}
	}

	// Set the team as the demo user's active tenant so newly created sites
	// are automatically associated with this team.
	if err := store.SetActiveTenantID(ctx, ownerID, team.ID); err != nil {
		slog.Warn("Failed to set active tenant", "error", err)
	}

	// Add extra users (created by seedAdditionalUsers) as team members.
	memberRoles := []struct {
		email string
		role  string
	}{
		{"bob@devtools.co", "admin"},
		{"diana@saaslaunch.com", "member"},
	}
	for _, m := range memberRoles {
		user, err := store.GetUserByEmail(ctx, m.email)
		if err != nil || user == nil {
			slog.Warn("Team member user not found, skipping", "email", m.email)
			continue
		}
		if err := store.AddTeamMember(ctx, team.ID, user.ID, m.role, ownerID); err != nil {
			slog.Warn("Failed to add team member", "email", m.email, "error", err)
		}
	}

	secondaryTeamName := "Northwind Studio"
	secondaryTeam, err := store.CreateTenant(ctx, ownerID, secondaryTeamName, "")
	if err != nil {
		teams, _, listErr := store.ListUserTeams(ctx, ownerID)
		if listErr == nil {
			for _, candidate := range teams {
				if strings.EqualFold(strings.TrimSpace(candidate.Name), secondaryTeamName) {
					c := candidate
					secondaryTeam = &c
					break
				}
			}
		}
	}
	if secondaryTeam != nil {
		slog.Info("Secondary demo team available", "name", secondaryTeamName, "id", secondaryTeam.ID)
	}

	if err := store.SetActiveTenantID(ctx, ownerID, team.ID); err != nil {
		slog.Warn("Failed to restore active tenant after seeding extra teams", "error", err, "team_id", team.ID)
	}

	slog.Info("Demo team seeded", "name", teamName, "id", team.ID, "members", len(memberRoles)+1)
}

func ensureSiteInActiveTeam(ctx context.Context, store *database.Store, userID uuid.UUID, domain string) (*api.Site, error) {
	normalized := strings.TrimSpace(strings.ToLower(domain))
	activeTenantID, err := store.GetActiveTenantID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve active tenant: %w", err)
	}

	var existing api.Site
	err = store.DB().QueryRowContext(ctx,
		`SELECT id, user_id, domain, data_retention_days, created_at
		 FROM sites
		 WHERE lower(domain) = ?
		 LIMIT 1`,
		normalized,
	).Scan(&existing.ID, &existing.UserID, &existing.Domain, &existing.DataRetentionDays, &existing.CreatedAt)
	if err == nil {
		if existing.UserID != userID {
			return nil, fmt.Errorf("domain %q already exists under a different user", domain)
		}

		if _, err := store.DB().ExecContext(ctx, `
			INSERT INTO site_tenants (site_id, tenant_id, created_at)
			VALUES (?, ?, NOW())
			ON CONFLICT (site_id) DO UPDATE SET
				tenant_id = excluded.tenant_id
		`, existing.ID, activeTenantID); err != nil {
			return nil, fmt.Errorf("rebind existing site to active tenant: %w", err)
		}

		slog.Info("Reusing existing demo site", "site_id", existing.ID, "domain", existing.Domain, "tenant_id", activeTenantID)
		return &existing, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("lookup existing site: %w", err)
	}

	sites, err := store.GetSites(ctx, userID)
	if err == nil {
		for _, site := range sites {
			if strings.EqualFold(strings.TrimSpace(site.Domain), normalized) {
				slog.Info("Reusing existing demo site", "site_id", site.ID, "domain", site.Domain)
				siteCopy := site
				return &siteCopy, nil
			}
		}
	}

	return store.CreateSite(ctx, userID, domain)
}

func hasAPIClientNamed(clients []api.APIClient, name string) bool {
	for _, client := range clients {
		if strings.EqualFold(strings.TrimSpace(client.Name), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func pickViewport(rng *mrand.Rand, kind string) (vw, vh, sw, sh int) {
	switch kind {
	case "mobile":
		v := mobileViewports[rng.Intn(len(mobileViewports))]
		return v.vw, v.vh, v.sw, v.sh
	case "tablet":
		v := tabletViewports[rng.Intn(len(tabletViewports))]
		return v.vw, v.vh, v.sw, v.sh
	default: // desktop
		v := desktopViewports[rng.Intn(len(desktopViewports))]
		return v.vw, v.vh, v.sw, v.sh
	}
}

func randomTimeInDay(rng *mrand.Rand, day time.Time) time.Time {
	// Traffic peaks 9-18 UTC (business hours weighted), some overnight traffic.
	hourWeights := []int{
		1, 1, 1, 1, 1, 2, // 0-5: very quiet
		4, 7, 10, 12, 14, 14, // 6-11: morning ramp
		15, 14, 13, 12, 11, 10, // 12-17: business hours
		8, 6, 5, 3, 2, 1, // 18-23: evening, then quiet
	}
	total := 0
	for _, w := range hourWeights {
		total += w
	}
	n := rng.Intn(total)
	hour := 0
	for i, w := range hourWeights {
		n -= w
		if n < 0 {
			hour = i
			break
		}
	}
	minute := rng.Intn(60)
	second := rng.Intn(60)
	return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second)
}

func randomCompanySize(rng *mrand.Rand) string {
	sizes := []string{"1-10", "11-50", "51-200", "201-500", "500+"}
	return sizes[rng.Intn(len(sizes))]
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func randomBilling(rng *mrand.Rand) string {
	if rng.Float64() < 0.65 {
		return "annual"
	}
	return "monthly"
}

func productPrice(product ecommerceProduct, billing string) int {
	if billing == "annual" && product.priceYear > 0 {
		return product.priceYear
	}
	return product.price
}

func randomCoupon(rng *mrand.Rand) string {
	return pickWeighted(rng, []weightedEntry[string]{
		{"", 55},
		{"SPRING25", 18},
		{"ANNUAL20", 15},
		{"WELCOME10", 12},
	})
}

func randomSignupSource(rng *mrand.Rand) string {
	return pickWeighted(rng, []weightedEntry[string]{
		{"footer-form", 40},
		{"blog-post", 25},
		{"exit-intent", 15},
		{"sidebar", 12},
		{"header-cta", 8},
	})
}

func randomNewsletterFormat(rng *mrand.Rand) string {
	return pickWeighted(rng, []weightedEntry[string]{
		{"weekly-digest", 50},
		{"product-updates", 30},
		{"tips-and-tricks", 20},
	})
}

func randomTrialPlan(rng *mrand.Rand) string {
	return pickWeighted(rng, []weightedEntry[string]{
		{"starter", 50},
		{"pro", 35},
		{"business", 15},
	})
}

func randomIndustry(rng *mrand.Rand) string {
	return pickWeighted(rng, []weightedEntry[string]{
		{"saas", 35},
		{"ecommerce", 20},
		{"media", 15},
		{"fintech", 12},
		{"healthcare", 8},
		{"education", 10},
	})
}

func randomDemoSource(rng *mrand.Rand) string {
	return pickWeighted(rng, []weightedEntry[string]{
		{"pricing-page", 40},
		{"demo-button", 30},
		{"contact-form", 20},
		{"live-chat", 10},
	})
}
