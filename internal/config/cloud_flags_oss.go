//go:build !billing

package config

import "flag"

func registerCloudFlags(
	_ *flag.FlagSet,
	_ *Config,
	_ func(string, string) string,
	_ func(string, int) int,
	_ func(string, int64) int64,
	_ func(string, bool) bool,
) {
}
