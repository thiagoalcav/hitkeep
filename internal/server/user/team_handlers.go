package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/entitlements"
	"hitkeep/internal/mailables"
	serverauth "hitkeep/internal/server/auth"
	"hitkeep/internal/server/shared"
)

func canManageTeam(role string) bool {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case database.TenantRoleOwner, database.TenantRoleAdmin:
		return true
	default:
		return false
	}
}

func (h *handler) appendTeamAudit(r *http.Request, teamID, actorID uuid.UUID, action, details string, targetUserID *uuid.UUID) {
	if err := h.ctx.Store.AppendTeamAuditEntry(r.Context(), teamID, actorID, action, details, targetUserID); err != nil {
		slog.Warn("Failed to append team audit entry", "error", err, "team_id", teamID, "actor_id", actorID, "action", action)
	}
}

func writeTeamActionError(w http.ResponseWriter, statusCode int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "error",
		"code":    code,
		"message": message,
	}); err != nil {
		slog.Error("Failed to encode team action error", "error", err, "code", code)
	}
}

func resolveTeamEntitlements(ctx context.Context, store *database.Store, provider entitlements.Provider, teamID uuid.UUID) *api.TeamEntitlements {
	if override := resolveCloudBillingTeamEntitlements(ctx, store, teamID); override != nil {
		return override
	}

	defaults := &entitlements.Entitlements{
		AllowSSO:            true,
		AllowCustomBranding: true,
	}

	ent := defaults
	if provider != nil {
		if resolved, err := provider.ForTenant(ctx, teamID); err == nil && resolved != nil {
			ent = resolved
		}
	}

	return &api.TeamEntitlements{
		MaxSitesPerTeam:     ent.MaxSitesPerTeam,
		MaxTeamMembers:      ent.MaxTeamMembers,
		MaxRetentionDays:    ent.MaxRetentionDays,
		AllowSSO:            ent.AllowSSO,
		AllowCustomBranding: ent.AllowCustomBranding,
	}
}

func resolveTeamPlan(ctx context.Context, store *database.Store, provider entitlements.Provider, teamID uuid.UUID) *api.TeamPlan {
	if override := resolveCloudBillingTeamPlan(ctx, store, teamID); override != nil {
		return override
	}

	describer, ok := provider.(entitlements.Describer)
	if !ok || describer == nil {
		return nil
	}

	plan, err := describer.DescribeTenant(ctx, teamID)
	if err != nil || plan == nil {
		return nil
	}
	if strings.TrimSpace(plan.Name) == "" && strings.TrimSpace(plan.Code) == "" {
		return nil
	}

	return &api.TeamPlan{
		Code:       strings.TrimSpace(plan.Code),
		Name:       strings.TrimSpace(plan.Name),
		UpgradeURL: strings.TrimSpace(plan.UpgradeURL),
		SupportURL: strings.TrimSpace(plan.SupportURL),
	}
}

func (h *handler) hydrateTeamSummaries(r *http.Request, teams []api.Team) []api.Team {
	if len(teams) == 0 {
		return teams
	}

	enriched := make([]api.Team, len(teams))
	copy(enriched, teams)
	for idx, team := range enriched {
		enriched[idx].Entitlements = resolveTeamEntitlements(r.Context(), h.ctx.Store, h.ctx.Entitlements, team.ID)
		enriched[idx].Plan = resolveTeamPlan(r.Context(), h.ctx.Store, h.ctx.Entitlements, team.ID)

		analyticsStore := h.ctx.Store
		if h.ctx.TenantStores != nil {
			store, err := h.ctx.TenantStores.ForTenant(r.Context(), team.ID)
			if err != nil {
				slog.Warn("Failed to resolve analytics store for team usage", "error", err, "team_id", team.ID)
				continue
			}
			analyticsStore = store
		}

		usage, err := h.ctx.Store.BuildTeamUsageSummary(r.Context(), team.ID, analyticsStore)
		if err != nil {
			slog.Warn("Failed to build team usage summary", "error", err, "team_id", team.ID)
			continue
		}
		enriched[idx].Usage = usage
	}

	return enriched
}

func (h *handler) sendTeamInviteEmail(r *http.Request, teamID, actorID uuid.UUID, invite *api.TeamInvite) {
	if h.ctx.Mailer == nil || invite == nil {
		return
	}

	inviteToken, err := h.ctx.Store.CreatePasswordResetToken(r.Context(), invite.Email)
	if err != nil {
		slog.Warn("Failed to create invite token for team invite", "error", err, "email", invite.Email, "team_id", teamID)
		return
	}

	teamName := "HitKeep Team"
	if team, err := h.ctx.Store.GetTenant(r.Context(), teamID); err == nil && team != nil {
		teamName = team.Name
	}
	inviterName := "Someone"
	if inviter, err := h.ctx.Store.GetUserByID(r.Context(), actorID); err == nil && inviter != nil {
		inviterName = inviter.Email
	}
	locale := "en"
	if recipient, err := h.ctx.Store.GetUserByEmail(r.Context(), invite.Email); err == nil && recipient != nil {
		if resolvedLocale, err := h.ctx.Store.GetUserLocale(r.Context(), recipient.ID); err == nil {
			locale = resolvedLocale
		}
	} else if actorID != uuid.Nil {
		if resolvedLocale, err := h.ctx.Store.GetUserLocale(r.Context(), actorID); err == nil && strings.TrimSpace(resolvedLocale) != "" {
			locale = resolvedLocale
		}
	}

	inviteLink := strings.TrimRight(h.ctx.Config.PublicURL, "/") + "/accept-invite?token=" + inviteToken
	if err := h.ctx.Mailer.Send(invite.Email, mailables.NewTeamInvite(inviteLink, teamName, inviterName, invite.Role, true, locale)); err != nil {
		slog.Warn("Failed to send team invite email", "error", err, "email", invite.Email, "team_id", teamID)
	}
}

func (h *handler) handleCreateTeam() http.HandlerFunc {
	type request struct {
		Name    string `json:"name"`
		LogoURL string `json:"logo_url"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			http.Error(w, "Team name is required", http.StatusBadRequest)
			return
		}
		if len(name) > 120 {
			http.Error(w, "Team name must be 120 characters or fewer", http.StatusBadRequest)
			return
		}

		logoURL := strings.TrimSpace(req.LogoURL)
		if logoURL != "" {
			if len(logoURL) > 2048 {
				http.Error(w, "Logo URL must be 2048 characters or fewer", http.StatusBadRequest)
				return
			}
			if _, err := url.ParseRequestURI(logoURL); err != nil {
				http.Error(w, "Invalid logo URL", http.StatusBadRequest)
				return
			}
		}

		if h.ctx.Config.CloudHosted {
			http.Error(w, "Managed cloud accounts are limited to one team", http.StatusForbidden)
			return
		}

		if h.ctx.Entitlements != nil {
			activeTenantID, entErr := h.ctx.Store.GetActiveTenantID(r.Context(), actorID)
			if entErr == nil {
				ent, entErr := h.ctx.Entitlements.ForTenant(r.Context(), activeTenantID)
				if entErr == nil && ent.MaxTeams > 0 {
					teams, _, _ := h.ctx.Store.ListUserTeams(r.Context(), actorID)
					if len(teams) >= ent.MaxTeams {
						http.Error(w, "Team limit reached", http.StatusForbidden)
						return
					}
				}
			}
		}

		team, err := h.ctx.Store.CreateTenant(r.Context(), actorID, name, logoURL)
		if err != nil {
			slog.Error("Failed to create team", "error", err, "actor_id", actorID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := h.ctx.Store.SetActiveTenantID(r.Context(), actorID, team.ID); err != nil {
			slog.Warn("Failed to auto-activate new team", "error", err, "team_id", team.ID, "actor_id", actorID)
		}
		h.appendTeamAudit(r, team.ID, actorID, "team.created", fmt.Sprintf("Team %q created", team.Name), nil)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]any{"team": team}); err != nil {
			slog.Error("Failed to encode create team response", "error", err, "actor_id", actorID)
		}
	}
}

func (h *handler) handleGetTeams() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teams, activeTeamID, err := h.ctx.Store.ListUserTeams(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to list teams", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resp := struct {
			ActiveTeamID  uuid.UUID   `json:"active_team_id"`
			RecentTeamIDs []uuid.UUID `json:"recent_team_ids"`
			Teams         []api.Team  `json:"teams"`
		}{
			ActiveTeamID:  activeTeamID,
			RecentTeamIDs: orderedRecentTeamIDs(teams, activeTeamID),
			Teams:         h.hydrateTeamSummaries(r, teams),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode teams response", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleSetActiveTeam() http.HandlerFunc {
	type request struct {
		TeamID string `json:"team_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(req.TeamID))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.SetActiveTenantID(r.Context(), userID, teamID); err != nil {
			if errors.Is(err, database.ErrTenantMembershipRequired) {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}
			slog.Error("Failed to set active team", "error", err, "user_id", userID, "team_id", teamID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		teams, activeTeamID, teamsErr := h.ctx.Store.ListUserTeams(r.Context(), userID)
		if teamsErr != nil {
			slog.Warn("Failed to load active team after active team update", "error", teamsErr, "user_id", userID, "team_id", teamID)
			teams = nil
			activeTeamID = teamID
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"active_team_id":  activeTeamID,
			"recent_team_ids": orderedRecentTeamIDs(teams, activeTeamID),
		}); err != nil {
			slog.Error("Failed to encode active team response", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleGetTeamMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		if _, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, userID); err != nil {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		members, err := h.ctx.Store.ListTeamMembers(r.Context(), teamID)
		if err != nil {
			slog.Error("Failed to list team members", "error", err, "user_id", userID, "team_id", teamID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(members); err != nil {
			slog.Error("Failed to encode team members response", "error", err, "user_id", userID, "team_id", teamID)
		}
	}
}

func (h *handler) handleGetTeamInvites() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		role, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, userID)
		if err != nil || !canManageTeam(role) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		invites, err := h.ctx.Store.ListTeamInvites(r.Context(), teamID)
		if err != nil {
			slog.Error("Failed to list team invites", "error", err, "user_id", userID, "team_id", teamID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(invites); err != nil {
			slog.Error("Failed to encode team invites response", "error", err, "user_id", userID, "team_id", teamID)
		}
	}
}

func (h *handler) handleGetTeamAudit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		role, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, userID)
		if err != nil || !canManageTeam(role) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		limit := 25
		if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
			parsedLimit, parseErr := strconv.Atoi(rawLimit)
			if parseErr != nil {
				http.Error(w, "Invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsedLimit
		}

		offset := 0
		if rawOffset := strings.TrimSpace(r.URL.Query().Get("offset")); rawOffset != "" {
			parsedOffset, parseErr := strconv.Atoi(rawOffset)
			if parseErr != nil {
				http.Error(w, "Invalid offset", http.StatusBadRequest)
				return
			}
			offset = parsedOffset
		}

		action := strings.TrimSpace(r.URL.Query().Get("action"))

		entries, total, err := h.ctx.Store.ListTeamAuditEntries(r.Context(), teamID, action, limit, offset)
		if err != nil {
			slog.Error("Failed to list team audit entries", "error", err, "user_id", userID, "team_id", teamID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(api.TeamAuditListResponse{
			Entries: entries,
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+len(entries) < total,
			Action:  action,
		}); err != nil {
			slog.Error("Failed to encode team audit response", "error", err, "user_id", userID, "team_id", teamID)
		}
	}
}

func (h *handler) handleAddTeamMember() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		actorRole, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, actorID)
		if err != nil || !canManageTeam(actorRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		parsedEmail, err := mail.ParseAddress(email)
		if err != nil || parsedEmail.Address != email {
			http.Error(w, "Invalid email", http.StatusBadRequest)
			return
		}

		role := strings.TrimSpace(strings.ToLower(req.Role))
		if role == "" {
			role = database.TenantRoleMember
		}
		if !database.IsValidTenantRole(role) {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}
		if !database.CanAssignTenantRole(actorRole, role) {
			http.Error(w, "Forbidden role assignment", http.StatusForbidden)
			return
		}
		if role == database.TenantRoleOwner {
			writeTeamActionError(w, http.StatusConflict, "ownership_transfer_required", "Use the ownership transfer action to assign the owner role")
			return
		}

		user, err := h.ctx.Store.GetUserByEmail(r.Context(), email)
		if err != nil {
			slog.Error("Failed to lookup invitee user", "error", err, "email", email, "team_id", teamID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var targetUserID uuid.UUID
		wasMember := false
		previousRole := ""
		if user != nil {
			targetUserID = user.ID

			isMember, err := h.ctx.Store.IsTenantMember(r.Context(), teamID, targetUserID)
			if err != nil {
				slog.Error("Failed to check team membership", "error", err, "email", email, "team_id", teamID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Prevent actors from re-inviting themselves (which would overwrite their own role).
			if isMember && targetUserID == actorID {
				http.Error(w, "Cannot change your own role", http.StatusConflict)
				return
			}
			if isMember {
				wasMember = true
				existingRole, roleErr := h.ctx.Store.GetTenantRole(r.Context(), teamID, targetUserID)
				if roleErr == nil {
					previousRole = existingRole
				}
			}
		} else {
			tempPassword := uuid.New().String()
			hashedPassword, err := serverauth.HashPassword(tempPassword)
			if err != nil {
				slog.Error("Failed to hash invitee password", "error", err, "email", email, "team_id", teamID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if h.ctx.Config.CloudHosted {
				targetUserID, err = h.ctx.Store.CreateUserWithoutDefaultTenant(r.Context(), email, hashedPassword)
			} else {
				targetUserID, err = h.ctx.Store.CreateUser(r.Context(), email, hashedPassword)
			}
			if err != nil {
				slog.Error("Failed to create invitee user", "error", err, "email", email, "team_id", teamID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		if h.ctx.Config.CloudHosted && !wasMember {
			teamCount, err := h.ctx.Store.CountUserNonDefaultTeams(r.Context(), targetUserID)
			if err != nil {
				slog.Error("Failed to count cloud invitee teams", "error", err, "email", email, "team_id", teamID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if teamCount > 0 {
				http.Error(w, "Managed cloud accounts are limited to one team", http.StatusConflict)
				return
			}

			pendingInvites, err := h.ctx.Store.ListPendingTeamInvitesByEmail(r.Context(), email)
			if err != nil {
				slog.Error("Failed to load pending cloud invites", "error", err, "email", email, "team_id", teamID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			for _, pendingInvite := range pendingInvites {
				if pendingInvite.TeamID != teamID {
					http.Error(w, "Managed cloud accounts are limited to one team", http.StatusConflict)
					return
				}
			}
		}

		if wasMember {
			if err := h.ctx.Store.AddTeamMember(r.Context(), teamID, targetUserID, role, actorID); err != nil {
				slog.Error("Failed to update team member", "error", err, "team_id", teamID, "target_user_id", targetUserID, "actor_id", actorID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			details := fmt.Sprintf("Member %s set to role %s", email, role)
			if previousRole != "" {
				details = fmt.Sprintf("Member %s role changed from %s to %s", email, previousRole, role)
			}
			targetID := targetUserID
			h.appendTeamAudit(r, teamID, actorID, "member.role_updated", details, &targetID)

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"status":    "ok",
				"is_invite": false,
			}); err != nil {
				slog.Error("Failed to encode add team member response", "error", err, "team_id", teamID, "actor_id", actorID)
			}
			return
		}

		invite, err := h.ctx.Store.CreateTeamInvite(r.Context(), teamID, email, role, &targetUserID, actorID)
		if err != nil {
			switch {
			case errors.Is(err, database.ErrTeamInviteAlreadyPending):
				http.Error(w, "Invite already pending", http.StatusConflict)
				return
			default:
				slog.Error("Failed to create team invite", "error", err, "team_id", teamID, "target_user_id", targetUserID, "actor_id", actorID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		details := fmt.Sprintf("Invitation sent to %s with role %s", email, role)
		targetID := targetUserID
		h.appendTeamAudit(r, teamID, actorID, "member.invited", details, &targetID)
		h.sendTeamInviteEmail(r, teamID, actorID, invite)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"is_invite": true,
			"invite":    invite,
		}); err != nil {
			slog.Error("Failed to encode add team member response", "error", err, "team_id", teamID, "actor_id", actorID)
		}
	}
}

func (h *handler) handleResendTeamInvite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}
		inviteID, err := uuid.Parse(strings.TrimSpace(r.PathValue("inviteId")))
		if err != nil {
			http.Error(w, "Invalid invite ID", http.StatusBadRequest)
			return
		}

		actorRole, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, actorID)
		if err != nil || !canManageTeam(actorRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		invite, err := h.ctx.Store.ResendTeamInvite(r.Context(), teamID, inviteID)
		if err != nil {
			switch {
			case errors.Is(err, database.ErrTeamInviteNotFound):
				http.Error(w, "Invite not found", http.StatusNotFound)
				return
			default:
				slog.Error("Failed to resend team invite", "error", err, "team_id", teamID, "invite_id", inviteID, "actor_id", actorID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		h.sendTeamInviteEmail(r, teamID, actorID, invite)
		h.appendTeamAudit(r, teamID, actorID, "member.invite_resent", fmt.Sprintf("Invitation resent to %s", invite.Email), invite.InvitedUserID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"status": "ok", "invite": invite}); err != nil {
			slog.Error("Failed to encode resend invite response", "error", err, "team_id", teamID, "invite_id", inviteID)
		}
	}
}

func (h *handler) handleRevokeTeamInvite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}
		inviteID, err := uuid.Parse(strings.TrimSpace(r.PathValue("inviteId")))
		if err != nil {
			http.Error(w, "Invalid invite ID", http.StatusBadRequest)
			return
		}

		actorRole, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, actorID)
		if err != nil || !canManageTeam(actorRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		invite, err := h.ctx.Store.GetTeamInvite(r.Context(), teamID, inviteID)
		if err != nil {
			switch {
			case errors.Is(err, database.ErrTeamInviteNotFound):
				http.Error(w, "Invite not found", http.StatusNotFound)
				return
			default:
				slog.Error("Failed to load team invite", "error", err, "team_id", teamID, "invite_id", inviteID, "actor_id", actorID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		if err := h.ctx.Store.RevokeTeamInvite(r.Context(), teamID, inviteID); err != nil {
			switch {
			case errors.Is(err, database.ErrTeamInviteNotFound):
				http.Error(w, "Invite not found", http.StatusNotFound)
				return
			default:
				slog.Error("Failed to revoke team invite", "error", err, "team_id", teamID, "invite_id", inviteID, "actor_id", actorID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		h.appendTeamAudit(r, teamID, actorID, "member.invite_revoked", fmt.Sprintf("Invitation revoked for %s", invite.Email), invite.InvitedUserID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode revoke invite response", "error", err, "team_id", teamID, "invite_id", inviteID)
		}
	}
}

func (h *handler) handleUpdateTeam() http.HandlerFunc {
	type request struct {
		Name    string `json:"name"`
		LogoURL string `json:"logo_url"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		actorRole, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, actorID)
		if err != nil || !canManageTeam(actorRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			http.Error(w, "Team name is required", http.StatusBadRequest)
			return
		}
		if len(name) > 120 {
			http.Error(w, "Team name must be 120 characters or fewer", http.StatusBadRequest)
			return
		}

		logoURL := strings.TrimSpace(req.LogoURL)
		if logoURL != "" {
			if len(logoURL) > 2048 {
				http.Error(w, "Logo URL must be 2048 characters or fewer", http.StatusBadRequest)
				return
			}
			if _, err := url.ParseRequestURI(logoURL); err != nil {
				http.Error(w, "Invalid logo URL", http.StatusBadRequest)
				return
			}
		}

		if err := h.ctx.Store.UpdateTenant(r.Context(), teamID, name, logoURL); err != nil {
			slog.Error("Failed to update team", "error", err, "team_id", teamID, "actor_id", actorID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		h.appendTeamAudit(r, teamID, actorID, "team.updated", fmt.Sprintf("Team settings updated (name=%q)", name), nil)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode update team response", "error", err, "team_id", teamID, "actor_id", actorID)
		}
	}
}

func (h *handler) handleTransferTeamOwnership() http.HandlerFunc {
	type request struct {
		TargetUserID string `json:"target_user_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		targetUserID, err := uuid.Parse(strings.TrimSpace(req.TargetUserID))
		if err != nil {
			http.Error(w, "Invalid target user ID", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.TransferTeamOwnership(r.Context(), teamID, actorID, targetUserID); err != nil {
			switch {
			case errors.Is(err, database.ErrTenantMembershipRequired), errors.Is(err, database.ErrTeamTransferRequiresOwner):
				writeTeamActionError(w, http.StatusForbidden, "ownership_transfer_forbidden", "Only team owners can transfer ownership")
				return
			case errors.Is(err, database.ErrTeamTransferTargetNotMember):
				writeTeamActionError(w, http.StatusBadRequest, "ownership_transfer_target_invalid", "The selected user must already be a team member")
				return
			case errors.Is(err, database.ErrTeamTransferSelf):
				writeTeamActionError(w, http.StatusConflict, "ownership_transfer_self", "Ownership transfer requires a different team member")
				return
			case errors.Is(err, database.ErrTeamTransferTargetAlreadyOwner):
				writeTeamActionError(w, http.StatusConflict, "ownership_transfer_already_owner", "The selected member is already an owner")
				return
			default:
				slog.Error("Failed to transfer team ownership", "error", err, "team_id", teamID, "actor_id", actorID, "target_user_id", targetUserID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		h.appendTeamAudit(r, teamID, actorID, "ownership.transferred", fmt.Sprintf("Ownership transferred to %s", targetUserID), &targetUserID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode ownership transfer response", "error", err, "team_id", teamID, "actor_id", actorID)
		}
	}
}

func (h *handler) handleArchiveTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.ArchiveTenant(r.Context(), teamID, actorID); err != nil {
			switch {
			case errors.Is(err, database.ErrTenantMembershipRequired), errors.Is(err, database.ErrTeamArchiveRequiresOwner):
				writeTeamActionError(w, http.StatusForbidden, "team_archive_forbidden", "Only team owners can archive this team")
				return
			case errors.Is(err, database.ErrTeamArchiveDefaultTenant):
				writeTeamActionError(w, http.StatusBadRequest, "team_archive_default_forbidden", "The default team cannot be archived")
				return
			case errors.Is(err, database.ErrTeamArchiveHasSites):
				writeTeamActionError(w, http.StatusBadRequest, "team_archive_has_sites", "Transfer or delete all sites before archiving this team")
				return
			default:
				slog.Error("Failed to archive team", "error", err, "team_id", teamID, "actor_id", actorID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		h.appendTeamAudit(r, teamID, actorID, "team.archived", fmt.Sprintf("Team %s archived", teamID), nil)

		teams, activeTeamID, teamsErr := h.ctx.Store.ListUserTeams(r.Context(), actorID)
		if teamsErr != nil {
			slog.Warn("Failed to load team list after archiving team", "error", teamsErr, "user_id", actorID, "team_id", teamID)
			teams = nil
			activeTeamID, _ = h.ctx.Store.GetActiveTenantID(r.Context(), actorID)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"active_team_id":  activeTeamID,
			"recent_team_ids": orderedRecentTeamIDs(teams, activeTeamID),
		}); err != nil {
			slog.Error("Failed to encode archive team response", "error", err, "team_id", teamID, "actor_id", actorID)
		}
	}
}

func (h *handler) handleRemoveTeamMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID := shared.GetUserIDFromContext(r)
		if actorID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}
		targetUserID, err := uuid.Parse(strings.TrimSpace(r.PathValue("userId")))
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		actorRole, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, actorID)
		if err != nil || !canManageTeam(actorRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		targetRole, err := h.ctx.Store.GetTenantRole(r.Context(), teamID, targetUserID)
		if err != nil {
			http.Error(w, "Team member not found", http.StatusNotFound)
			return
		}
		if !database.CanAssignTenantRole(actorRole, targetRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		ownerCount, err := h.ctx.Store.CountTeamOwners(r.Context(), teamID)
		if err != nil {
			slog.Error("Failed to count team owners", "error", err, "team_id", teamID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if targetRole == database.TenantRoleOwner && ownerCount <= 1 {
			http.Error(w, "Cannot remove last owner", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.RemoveTeamMember(r.Context(), teamID, targetUserID); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "Team member not found", http.StatusNotFound)
				return
			}
			slog.Error("Failed to remove team member", "error", err, "team_id", teamID, "target_user_id", targetUserID, "actor_id", actorID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		removedUserID := targetUserID
		h.appendTeamAudit(r, teamID, actorID, "member.removed", fmt.Sprintf("Member %s removed", targetUserID), &removedUserID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode remove team member response", "error", err, "team_id", teamID, "actor_id", actorID)
		}
	}
}

func (h *handler) handleLeaveTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid team ID", http.StatusBadRequest)
			return
		}

		nextActiveTeamID, err := h.ctx.Store.LeaveTeam(r.Context(), teamID, userID)
		if err != nil {
			switch {
			case errors.Is(err, database.ErrTenantMembershipRequired):
				writeTeamActionError(w, http.StatusForbidden, "team_membership_required", "Access denied")
				return
			case errors.Is(err, database.ErrTeamLastOwner):
				writeTeamActionError(w, http.StatusBadRequest, "team_last_owner", "Cannot leave as the last owner")
				return
			case errors.Is(err, database.ErrUserOnlyTeam):
				writeTeamActionError(w, http.StatusBadRequest, "user_only_team", "Cannot leave your only team")
				return
			default:
				slog.Error("Failed to leave team", "error", err, "user_id", userID, "team_id", teamID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		leftUserID := userID
		h.appendTeamAudit(r, teamID, userID, "member.left", fmt.Sprintf("Member %s left the team", userID), &leftUserID)

		teams, activeTeamID, teamsErr := h.ctx.Store.ListUserTeams(r.Context(), userID)
		if teamsErr != nil {
			slog.Warn("Failed to load team list after leaving team", "error", teamsErr, "user_id", userID, "team_id", teamID)
			teams = nil
			activeTeamID = nextActiveTeamID
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"active_team_id":  activeTeamID,
			"recent_team_ids": orderedRecentTeamIDs(teams, activeTeamID),
		}); err != nil {
			slog.Error("Failed to encode leave team response", "error", err, "user_id", userID, "team_id", teamID)
		}
	}
}

func orderedRecentTeamIDs(teams []api.Team, activeTeamID uuid.UUID) []uuid.UUID {
	recent := make([]uuid.UUID, 0, len(teams))
	if activeTeamID != uuid.Nil {
		recent = append(recent, activeTeamID)
	}
	for _, team := range teams {
		if team.ID == uuid.Nil || team.ID == activeTeamID {
			continue
		}
		recent = append(recent, team.ID)
	}
	return recent
}
