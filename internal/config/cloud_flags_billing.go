//go:build billing

package config

import "flag"

func registerCloudFlags(
	fs *flag.FlagSet,
	conf *Config,
	getEnv func(string, string) string,
	getInt func(string, int) int,
	getInt64 func(string, int64) int64,
	getBool func(string, bool) bool,
) {
	defCloudHosted := getBool("HITKEEP_CLOUD_HOSTED", true)
	defCloudSignupEnabled := getBool("HITKEEP_CLOUD_SIGNUP_ENABLED", false)
	defCloudJurisdiction := getEnv("HITKEEP_CLOUD_JURISDICTION", "")
	defCloudRegion := getEnv("HITKEEP_CLOUD_REGION", "")
	defCloudUpgradeURL := getEnv("HITKEEP_CLOUD_UPGRADE_URL", "")
	defCloudSupportURL := getEnv("HITKEEP_CLOUD_SUPPORT_URL", "")
	defCloudPlanCode := getEnv("HITKEEP_CLOUD_PLAN_CODE", "free")
	defCloudPlanName := getEnv("HITKEEP_CLOUD_PLAN_NAME", "Free")
	defCloudMaxTeams := getInt("HITKEEP_CLOUD_MAX_TEAMS", 1)
	defCloudMaxSitesPerTeam := getInt("HITKEEP_CLOUD_MAX_SITES_PER_TEAM", 3)
	defCloudMaxRetentionDays := getInt("HITKEEP_CLOUD_MAX_RETENTION_DAYS", 60)
	defCloudMaxTeamMembers := getInt("HITKEEP_CLOUD_MAX_TEAM_MEMBERS", 3)
	defCloudAllowSSO := getBool("HITKEEP_CLOUD_ALLOW_SSO", false)
	defCloudAllowCustomBranding := getBool("HITKEEP_CLOUD_ALLOW_CUSTOM_BRANDING", false)
	defStripeSecretKey := getEnv("HITKEEP_STRIPE_SECRET_KEY", "")
	defStripePublishableKey := getEnv("HITKEEP_STRIPE_PUBLISHABLE_KEY", "")
	defStripeWebhookSecret := getEnv("HITKEEP_STRIPE_WEBHOOK_SECRET", "")
	defStripePortalConfigurationID := getEnv("HITKEEP_STRIPE_PORTAL_CONFIGURATION_ID", "")
	defStripePriceProMonthly := getEnv("HITKEEP_STRIPE_PRICE_PRO_MONTHLY", "")
	defStripePriceBusinessMonthly := getEnv("HITKEEP_STRIPE_PRICE_BUSINESS_MONTHLY", "")
	defCloudCheckoutSuccessURL := getEnv("HITKEEP_CLOUD_CHECKOUT_SUCCESS_URL", "")
	defCloudCheckoutCancelURL := getEnv("HITKEEP_CLOUD_CHECKOUT_CANCEL_URL", "")

	fs.BoolVar(&conf.CloudHosted, "cloud-hosted", defCloudHosted, "Enable managed cloud runtime surfaces")
	fs.BoolVar(&conf.CloudSignupEnabled, "cloud-signup-enabled", defCloudSignupEnabled, "Enable hosted self-serve onboarding surfaces")
	fs.StringVar(&conf.CloudJurisdiction, "cloud-jurisdiction", defCloudJurisdiction, "Managed cloud jurisdiction label")
	fs.StringVar(&conf.CloudRegion, "cloud-region", defCloudRegion, "Managed cloud region label")
	fs.StringVar(&conf.CloudUpgradeURL, "cloud-upgrade-url", defCloudUpgradeURL, "Managed cloud upgrade URL")
	fs.StringVar(&conf.CloudSupportURL, "cloud-support-url", defCloudSupportURL, "Managed cloud support URL")
	fs.StringVar(&conf.CloudPlanCode, "cloud-plan-code", defCloudPlanCode, "Managed cloud plan code")
	fs.StringVar(&conf.CloudPlanName, "cloud-plan-name", defCloudPlanName, "Managed cloud plan name")
	fs.IntVar(&conf.CloudMaxTeams, "cloud-max-teams", defCloudMaxTeams, "Managed cloud max teams per user")
	fs.IntVar(&conf.CloudMaxSitesPerTeam, "cloud-max-sites-per-team", defCloudMaxSitesPerTeam, "Managed cloud max sites per team")
	fs.IntVar(&conf.CloudMaxRetentionDays, "cloud-max-retention-days", defCloudMaxRetentionDays, "Managed cloud max retention days")
	fs.IntVar(&conf.CloudMaxTeamMembers, "cloud-max-team-members", defCloudMaxTeamMembers, "Managed cloud max members per team")
	fs.BoolVar(&conf.CloudAllowSSO, "cloud-allow-sso", defCloudAllowSSO, "Managed cloud SSO entitlement")
	fs.BoolVar(&conf.CloudAllowCustomBranding, "cloud-allow-custom-branding", defCloudAllowCustomBranding, "Managed cloud custom branding entitlement")
	fs.StringVar(&conf.StripeSecretKey, "stripe-secret-key", defStripeSecretKey, "Stripe secret key for managed cloud billing")
	fs.StringVar(&conf.StripePublishableKey, "stripe-publishable-key", defStripePublishableKey, "Stripe publishable key for managed cloud billing")
	fs.StringVar(&conf.StripeWebhookSecret, "stripe-webhook-secret", defStripeWebhookSecret, "Stripe webhook signing secret for managed cloud billing")
	fs.StringVar(&conf.StripePortalConfigurationID, "stripe-portal-configuration-id", defStripePortalConfigurationID, "Stripe customer portal configuration ID for managed cloud billing")
	fs.StringVar(&conf.StripePriceProMonthly, "stripe-price-pro-monthly", defStripePriceProMonthly, "Stripe monthly recurring price ID for the Pro plan")
	fs.StringVar(&conf.StripePriceBusinessMonthly, "stripe-price-business-monthly", defStripePriceBusinessMonthly, "Stripe monthly recurring price ID for the Business plan")
	fs.StringVar(&conf.CloudCheckoutSuccessURL, "cloud-checkout-success-url", defCloudCheckoutSuccessURL, "Managed cloud checkout success URL override")
	fs.StringVar(&conf.CloudCheckoutCancelURL, "cloud-checkout-cancel-url", defCloudCheckoutCancelURL, "Managed cloud checkout cancel URL override")
}
