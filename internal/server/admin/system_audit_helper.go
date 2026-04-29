package admin

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func (h *handler) appendAudit(r *http.Request, action, targetType, targetID, targetLabel, outcome, details string) {
	ctx := r.Context()

	actorID := shared.GetUserIDFromContext(r)
	if actorID == uuid.Nil {
		slog.Warn("Instance audit without actor ID", "action", action)
		return
	}

	actorEmail := ""
	actorRole := ""
	permCtx, ok := ctx.Value(shared.PermissionKey).(shared.PermissionContext)
	if ok {
		actorRole = string(permCtx.InstanceRole)
	}

	user, err := h.ctx.Store.GetUserByID(ctx, actorID)
	if err == nil && user != nil {
		actorEmail = user.Email
	}
	if actorRole == "" {
		role, err := h.ctx.Store.GetInstanceRole(ctx, actorID)
		if err == nil {
			actorRole = string(role)
		}
	}

	ipAddress := shared.GetRealIP(r, h.ctx.Config.GetTrustedProxyNetworks())

	params := database.InstanceAuditParams{
		ActorID:     actorID,
		ActorEmail:  actorEmail,
		ActorRole:   actorRole,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		TargetLabel: targetLabel,
		Outcome:     outcome,
		IPAddress:   ipAddress,
		UserAgent:   strings.TrimSpace(r.UserAgent()),
		RequestID:   strings.TrimSpace(r.Header.Get("X-Request-Id")),
		Details:     details,
	}

	if err := h.ctx.Store.AppendInstanceAuditEntry(ctx, params); err != nil {
		slog.Error("Failed to append instance audit entry", "error", err, "action", action)
	}
}
