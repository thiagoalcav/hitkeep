//go:build !billing

package system

import "hitkeep/internal/api"

func (h *handler) cloudStatus() *api.CloudStatus {
	return nil
}
