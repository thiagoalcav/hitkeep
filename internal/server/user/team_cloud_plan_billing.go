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

	return &api.TeamPlan{
		Code: strings.TrimSpace(account.PlanCode),
		Name: strings.TrimSpace(account.PlanName),
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

	switch strings.TrimSpace(account.PlanCode) {
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
