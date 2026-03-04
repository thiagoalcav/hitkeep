package mailables

import (
	"hitkeep/internal/mailer"
)

// TeamInvite implements the mailer.Mailable interface for team membership invites.
type TeamInvite struct {
	Link                 string
	TeamName             string
	Inviter              string
	Role                 string
	RequiresAccountSetup bool
}

func NewTeamInvite(link, teamName, inviter, role string, requiresAccountSetup bool) mailer.Mailable {
	return &TeamInvite{
		Link:                 link,
		TeamName:             teamName,
		Inviter:              inviter,
		Role:                 role,
		RequiresAccountSetup: requiresAccountSetup,
	}
}

func (m *TeamInvite) Subject() string {
	if m.RequiresAccountSetup {
		return "You're invited to join " + m.TeamName + " on HitKeep"
	}
	return "You've been added to " + m.TeamName + " on HitKeep"
}

func (m *TeamInvite) Template() string {
	return "team_invite.mjml"
}

func (m *TeamInvite) Data() any {
	actionLabel := "Open HitKeep"
	helpText := "Sign in to access your team workspace."
	if m.RequiresAccountSetup {
		actionLabel = "Set Password & Join Team"
		helpText = "Create your password to activate your account and join this team."
	}

	return struct {
		Link                 string
		TeamName             string
		Inviter              string
		Role                 string
		ActionLabel          string
		HelpText             string
		RequiresAccountSetup bool
	}{
		Link:                 m.Link,
		TeamName:             m.TeamName,
		Inviter:              m.Inviter,
		Role:                 m.Role,
		ActionLabel:          actionLabel,
		HelpText:             helpText,
		RequiresAccountSetup: m.RequiresAccountSetup,
	}
}
