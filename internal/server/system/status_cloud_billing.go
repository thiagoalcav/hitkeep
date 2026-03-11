//go:build billing

package system

import (
	"strings"

	"hitkeep/internal/api"
)

func (h *handler) cloudStatus() *api.CloudStatus {
	if h.ctx == nil || h.ctx.Config == nil || !h.ctx.Config.CloudHosted {
		return nil
	}

	return &api.CloudStatus{
		Hosted:        true,
		SignupEnabled: h.ctx.Config.CloudSignupEnabled,
		Jurisdiction:  strings.TrimSpace(h.ctx.Config.CloudJurisdiction),
		Region:        strings.TrimSpace(h.ctx.Config.CloudRegion),
		UpgradeURL:    strings.TrimSpace(h.ctx.Config.CloudUpgradeURL),
		SupportURL:    strings.TrimSpace(h.ctx.Config.CloudSupportURL),
	}
}
