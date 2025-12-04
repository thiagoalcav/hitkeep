package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
	"hitkeep/internal/mailables"
)

func (s *Server) handleListUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement ListUsers in store_user.go if not exists
		// For now, let's assume it exists or we need to add it.
		// Based on previous file reads, ListUsers wasn't in store_user.go.
		// I'll need to add it. For now, I'll stub it or use a placeholder.

		// Actually, I should add ListUsers to store_user.go first.
		// But let's write the handler assuming it will be there.

		users, err := s.store.ListUsers(r.Context())
		if err != nil {
			slog.Error("Failed to list users", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(users); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleUpdateUserRole() http.HandlerFunc {
	type request struct {
		Role string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		targetUserIDStr := r.PathValue("id")
		targetUserID, err := uuid.Parse(targetUserIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		actorID := getUserIDFromContext(r)

		err = s.store.UpdateInstanceRole(r.Context(), targetUserID, auth.InstanceRole(req.Role), actorID)
		if err != nil {
			slog.Error("Failed to update role", "error", err)
			http.Error(w, "Failed to update role", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetUserIDStr := r.PathValue("id")
		targetUserID, err := uuid.Parse(targetUserIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		actorID := getUserIDFromContext(r)

		// Prevent deleting yourself
		if actorID == targetUserID {
			http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
			return
		}

		// Check if target user is an owner (optional safety check, though role check handles permissions)
		// Ideally, only owners can delete other owners, etc.
		// For now, we rely on the route permission check (PermInstanceManageUsers).

		err = s.store.DeleteUser(r.Context(), targetUserID)
		if err != nil {
			slog.Error("Failed to delete user", "error", err)
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleAdminListSites() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sites, err := s.store.ListAllSites(r.Context())
		if err != nil {
			slog.Error("Failed to list all sites", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(sites); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleAdminDeleteSite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		err = s.store.DeleteSite(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to delete site", "error", err)
			http.Error(w, "Failed to delete site", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleGetSiteMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		members, err := s.store.GetSiteMembers(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to get members", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(members); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleAddSiteMember() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Find user by email
		user, err := s.store.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			slog.Error("Database error checking user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// If user doesn't exist, create them (simplified flow)
		var userID uuid.UUID
		var isNewUser bool
		var inviteToken string

		if user == nil {
			// Create a placeholder user. In a real system, this would trigger an invite email.
			// For now, we create a user with a random password that they can reset later.
			// Or better, we just create the user record.
			// Since CreateUser requires a password, we'll generate a random one.
			tempPassword := uuid.New().String() // Temporary
			hashedPassword, err := hashPassword(tempPassword)
			if err != nil {
				slog.Error("Failed to hash password", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			userID, err = s.store.CreateUser(r.Context(), req.Email, hashedPassword)
			if err != nil {
				slog.Error("Failed to create user", "error", err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
			isNewUser = true

			// Generate invite token (reusing password reset token mechanism)
			inviteToken, err = s.store.CreatePasswordResetToken(r.Context(), req.Email)
			if err != nil {
				slog.Error("Failed to create invite token", "error", err)
				// Continue anyway, user is created but won't get email.
				// They can use forgot password later.
			}
		} else {
			userID = user.ID
		}

		actorID := getUserIDFromContext(r)

		err = s.store.AddSiteMember(r.Context(), siteID, userID, auth.SiteRole(req.Role), actorID)
		if err != nil {
			slog.Error("Failed to add member", "error", err)
			http.Error(w, "Failed to add member", http.StatusInternalServerError)
			return
		}

		// Send invite email if new user
		if isNewUser && inviteToken != "" {
			// Get site details for email
			site, err := s.store.GetSite(r.Context(), siteID, actorID)
			siteName := "Unknown Site"
			if err == nil && site != nil {
				siteName = site.Domain
			}

			// Get inviter details
			inviter, err := s.store.GetUserByID(r.Context(), actorID)
			inviterName := "Someone"
			if err == nil && inviter != nil {
				inviterName = inviter.Email
			}

			inviteLink := s.conf.PublicURL + "/accept-invite?token=" + inviteToken
			err = s.mailer.Send(req.Email, mailables.NewUserInvite(inviteLink, siteName, inviterName))
			if err != nil {
				slog.Warn("Failed to send invite email", "error", err, "email", req.Email)
				// Don't fail the request, just log warning
			}
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleRemoveSiteMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		userIDStr := r.PathValue("userId")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		actorID := getUserIDFromContext(r)

		// Can't remove yourself if you're the only owner
		if actorID == userID {
			role, _ := s.store.GetSiteRole(r.Context(), userID, siteID)
			if role == auth.SiteOwner {
				owners, _ := s.store.CountSiteOwners(r.Context(), siteID)
				if owners <= 1 {
					http.Error(w, "Cannot remove the last owner", http.StatusBadRequest)
					return
				}
			}
		}

		err = s.store.RemoveSiteMember(r.Context(), siteID, userID, actorID)
		if err != nil {
			slog.Error("Failed to remove member", "error", err)
			http.Error(w, "Failed to remove member", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}
