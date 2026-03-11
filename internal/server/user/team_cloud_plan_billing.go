//go:build billing

package user

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

func resolveCloudBillingTeamPlan(ctx context.Context, store *database.Store, teamID uuid.UUID) *api.TeamPlan {
	if store == nil {
		return nil
	}

	account, err := store.GetCloudBillingAccount(ctx, teamID)
	if errors.Is(err, database.ErrCloudBillingAccountNotFound) || account == nil {
		return nil
	}
	if err != nil {
		return nil
	}

	code, name := effectiveCloudBillingPlan(account)

	return &api.TeamPlan{
		Code: code,
		Name: name,
	}
}

func resolveCloudBillingTeamEntitlements(ctx context.Context, store *database.Store, teamID uuid.UUID) *api.TeamEntitlements {
	if store == nil {
		return nil
	}

	account, err := store.GetCloudBillingAccount(ctx, teamID)
	if errors.Is(err, database.ErrCloudBillingAccountNotFound) || account == nil {
		return nil
	}
	if err != nil {
		return nil
	}

	code, _ := effectiveCloudBillingPlan(account)

	switch code {
	case database.CloudPlanBusiness:
		return &api.TeamEntitlements{
			MaxSitesPerTeam:     50,
			MaxTeamMembers:      20,
			MaxMonthlyEvents:    1000000,
			MaxRetentionDays:    1095,
			AllowSSO:            true,
			AllowCustomBranding: true,
		}
	case database.CloudPlanPro:
		return &api.TeamEntitlements{
			MaxSitesPerTeam:     10,
			MaxTeamMembers:      5,
			MaxMonthlyEvents:    100000,
			MaxRetentionDays:    365,
			AllowSSO:            false,
			AllowCustomBranding: false,
		}
	case database.CloudPlanFree:
		return &api.TeamEntitlements{
			MaxSitesPerTeam:     3,
			MaxTeamMembers:      3,
			MaxMonthlyEvents:    10000,
			MaxRetentionDays:    60,
			AllowSSO:            false,
			AllowCustomBranding: false,
		}
	default:
		return nil
	}
}

func effectiveCloudBillingPlan(account *database.CloudBillingAccount) (string, string) {
	if account == nil {
		return "", ""
	}

	switch strings.TrimSpace(account.SubscriptionStatus) {
	case "", database.CloudSubscriptionStatusFree, "pending_checkout", "canceled", database.CloudSubscriptionStatusChargebackLost:
		return database.CloudPlanFree, "Free"
	default:
		return strings.TrimSpace(account.PlanCode), strings.TrimSpace(account.PlanName)
	}
}
