//go:build billing

package entitlements

import "hitkeep/internal/config"

func NewProvider(conf *config.Config) Provider {
	if conf == nil || !conf.CloudHosted {
		return NewDefaultProvider()
	}

	return NewStaticProvider(Entitlements{
		MaxTeams:            conf.CloudMaxTeams,
		MaxSitesPerTeam:     conf.CloudMaxSitesPerTeam,
		MaxRetentionDays:    conf.CloudMaxRetentionDays,
		MaxTeamMembers:      conf.CloudMaxTeamMembers,
		AllowSSO:            conf.CloudAllowSSO,
		AllowCustomBranding: conf.CloudAllowCustomBranding,
	}, PlanInfo{
		Code:       conf.CloudPlanCode,
		Name:       conf.CloudPlanName,
		UpgradeURL: conf.CloudUpgradeURL,
		SupportURL: conf.CloudSupportURL,
	})
}
