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
//   - Sets up Goals and Funnels that reference those events.
//   - Runs the rollup backfill so all charts are populated immediately.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/worker"
)

// ptr returns a pointer to v. Avoids verbose &local boilerplate for optional fields.
func ptr[T any](v T) *T { return &v }

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
	{ptr("https://www.google.com"), 200},             // Google search
	{ptr("https://news.ycombinator.com"), 120},       // HN
	{ptr("https://twitter.com"), 80},                 // Twitter/X
	{ptr("https://www.reddit.com/r/selfhosted"), 60}, // Reddit
	{ptr("https://www.linkedin.com"), 50},            // LinkedIn
	{ptr("https://github.com/hitkeep/hitkeep"), 50},  // GitHub
	{ptr("https://dev.to"), 40},                      // dev.to
	{ptr("https://lobste.rs"), 30},                   // Lobsters
	{ptr("https://www.producthunt.com"), 20},         // Product Hunt
}

var countries = []weightedEntry[*string]{
	{ptr("US"), 350},
	{ptr("DE"), 100},
	{ptr("GB"), 80},
	{ptr("CA"), 60},
	{ptr("FR"), 50},
	{ptr("NL"), 50},
	{ptr("AU"), 40},
	{ptr("SE"), 30},
	{ptr("CH"), 30},
	{ptr("IN"), 35},
	{ptr("JP"), 25},
	{ptr("BR"), 25},
	{ptr("PL"), 20},
	{ptr("ES"), 20},
	{ptr("IT"), 20},
	{ptr("NO"), 15},
	{ptr("FI"), 15},
	{ptr("DK"), 15},
	{ptr("SG"), 15},
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
	{ptr("en-US"), 380},
	{ptr("en-GB"), 80},
	{ptr("de-DE"), 100},
	{ptr("fr-FR"), 50},
	{ptr("nl-NL"), 50},
	{ptr("sv-SE"), 30},
	{ptr("pt-BR"), 25},
	{ptr("ja-JP"), 25},
	{ptr("es-ES"), 20},
	{ptr("pl-PL"), 20},
	{ptr("it-IT"), 20},
}

type utmParams struct {
	source, medium, campaign string
	term, content            *string
}

var utmCampaigns = []weightedEntry[*utmParams]{
	{nil, 800}, // no UTM (organic / direct)
	{&utmParams{
		source: "twitter", medium: "social", campaign: "product-launch-2025",
	}, 40},
	{&utmParams{
		source: "google", medium: "cpc", campaign: "self-hosted-analytics",
		term: ptr("self hosted analytics"), content: ptr("headline-a"),
	}, 35},
	{&utmParams{
		source: "google", medium: "cpc", campaign: "self-hosted-analytics",
		term: ptr("open source analytics"), content: ptr("headline-b"),
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

// ─────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────

func main() {
	dbPath := flag.String("db", "hitkeep.db", "Path to hitkeep.db")
	email := flag.String("email", "demo@example.com", "Demo user email")
	password := flag.String("password", "demo1234", "Demo user password")
	days := flag.Int("days", 90, "Days of demo traffic to generate")
	domain := flag.String("domain", "acme-analytics.io", "Demo site domain")
	seed := flag.Int64("seed", 42, "Random seed for reproducibility")
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

	rng := mrand.New(mrand.NewSource(*seed)) // #nosec G404 — demo data seeding; reproducibility trumps entropy

	// Seed extra users first so that the demo site (created below) is the
	// most-recently-created entry. GetSites orders by created_at DESC, so the
	// first item in the list — which the UI auto-selects — will be the demo
	// site that actually has traffic data.
	seedAdditionalUsers(ctx, store)

	slog.Info("Creating demo user", "email", *email)
	userID := ensureUser(ctx, store, *email, *password)

	slog.Info("Creating demo site", "domain", *domain)
	site, err := store.CreateSite(ctx, userID, *domain)
	if err != nil {
		slog.Error("Failed to create site", "error", err)
		os.Exit(1)
	}
	siteID := site.ID
	slog.Info("Site created", "site_id", siteID)

	goalIDs := createGoals(ctx, store, siteID)
	createFunnels(ctx, store, siteID)

	slog.Info("Seeding traffic", "days", *days)
	stats := seedTraffic(ctx, store, siteID, goalIDs, *days, rng)

	slog.Info("Running rollup backfill...")
	rollupWorker := worker.NewRollupBackfillWorker(store)
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

	for d := 0; d < numDays; d++ {
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

		dailyHits := int(float64(base) * growth * variation)
		if dailyHits < 10 {
			dailyHits = 10
		}

		// Distribute hits across the day in sessions of 1-5 pages.
		hitsLeft := dailyHits
		for hitsLeft > 0 {
			sessionLen := 1 + rng.Intn(5)
			if sessionLen > hitsLeft {
				sessionLen = hitsLeft
			}
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

			for i := 0; i < sessionLen; i++ {
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
					UserAgent:      ptr(uaEntry.ua),
					CountryCode:    country,
					Language:       lang,
					ViewportWidth:  ptr(vw),
					ViewportHeight: ptr(vh),
					ScreenWidth:    ptr(sw),
					ScreenHeight:   ptr(sh),
					IsUnique:       ptr(i == 0), // only the first hit in a session is unique
				}

				// Referrer only on the first hit of a session.
				if i == 0 {
					h.Referrer = ref
				}

				// UTM params only on the first hit.
				if i == 0 && utmEntry != nil {
					h.UTMSource = ptr(utmEntry.source)
					h.UTMMedium = ptr(utmEntry.medium)
					h.UTMCampaign = ptr(utmEntry.campaign)
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
			events := fireConversionEvents(ctx, store, siteID, sessionID, goals, rng, sessionStart.Add(time.Duration(sessionLen*90+30)*time.Second))
			stats.events += events
		}

		if d%10 == 0 || d == numDays-1 {
			slog.Info("Progress", "day", d+1, "of", numDays, "hits_so_far", stats.hits)
		}
	}

	return stats
}

// fireConversionEvents randomly fires zero or more conversion events for a session.
func fireConversionEvents(ctx context.Context, store *database.Store, siteID, sessionID uuid.UUID, goals goalIDs, rng *mrand.Rand, ts time.Time) int {
	_ = goals // goalIDs are used for reference, events use the same string values
	count := 0

	type conversionEvent struct {
		prob  float64
		name  string
		props map[string]any
	}

	conversions := []conversionEvent{
		{0.030, "newsletter_signup", map[string]any{"source": "footer-form"}},
		{0.020, "trial_started", map[string]any{"plan": "free-trial", "billing": "monthly"}},
		{0.015, "demo_requested", map[string]any{"company_size": randomCompanySize(rng)}},
		{0.008, "purchase_completed", map[string]any{
			"plan":     randomPlan(rng),
			"billing":  randomBilling(rng),
			"amount":   randomAmount(rng),
			"currency": "USD",
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
	return count
}

// ─────────────────────────────────────────────
// Additional users / sites (admin panel population)
// ─────────────────────────────────────────────

// seedAdditionalUsers creates extra users and their sites so that the admin
// panel looks populated in screenshots. No traffic is generated for them.
func seedAdditionalUsers(ctx context.Context, store *database.Store) {
	extra := []struct{ email, domain string }{
		{"alice@techblog.io", "techblog.io"},
		{"bob@devtools.co", "devtools.co"},
		{"charlie@startup.app", "startup.app"},
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

func randomPlan(rng *mrand.Rand) string {
	plans := []string{"starter", "pro", "business", "enterprise"}
	weights := []int{40, 35, 20, 5}
	return pickWeighted(rng, []weightedEntry[string]{
		{plans[0], weights[0]},
		{plans[1], weights[1]},
		{plans[2], weights[2]},
		{plans[3], weights[3]},
	})
}

func randomBilling(rng *mrand.Rand) string {
	if rng.Float64() < 0.65 {
		return "annual"
	}
	return "monthly"
}

func randomAmount(rng *mrand.Rand) int {
	amounts := []int{29, 79, 199, 499}
	weights := []int{40, 35, 20, 5}
	return pickWeighted(rng, []weightedEntry[int]{
		{amounts[0], weights[0]},
		{amounts[1], weights[1]},
		{amounts[2], weights[2]},
		{amounts[3], weights[3]},
	})
}
