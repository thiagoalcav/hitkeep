//go:build !billing

package user

import (
	"context"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

func resolveCloudBillingTeamPlan(_ context.Context, _ *database.Store, _ uuid.UUID) *api.TeamPlan {
	return nil
}

func resolveCloudBillingTeamEntitlements(_ context.Context, _ *database.Store, _ uuid.UUID) *api.TeamEntitlements {
	return nil
}
