//go:build !billing

package entitlements

import "hitkeep/internal/config"

func NewProvider(_ *config.Config) Provider {
	return NewDefaultProvider()
}
