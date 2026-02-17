package blocking

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
)

const defaultRefreshInterval = 30 * time.Second

type IPFilter struct {
	store *database.Store

	// Map site ID to blocked networks.
	siteRules map[uuid.UUID][]*net.IPNet
	// Global blocked networks.
	globalRules []*net.IPNet

	mu sync.RWMutex
}

func NewIPFilter(store *database.Store) *IPFilter {
	return &IPFilter{
		store:     store,
		siteRules: make(map[uuid.UUID][]*net.IPNet),
	}
}

// StartRefreshLoop updates in-memory rules every 30 seconds.
func (f *IPFilter) StartRefreshLoop(ctx context.Context) {
	if f.store == nil {
		return
	}

	if err := f.Refresh(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Failed initial IP exclusion load", "error", err)
	}

	go func() {
		ticker := time.NewTicker(defaultRefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := f.Refresh(ctx); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("Failed to refresh IP exclusions", "error", err)
				}
			}
		}
	}()
}

func (f *IPFilter) Refresh(ctx context.Context) error {
	instanceCIDRs, err := f.store.ListInstanceExclusionCIDRs(ctx)
	if err != nil {
		return err
	}

	siteCIDRs, err := f.store.ListSiteExclusionCIDRs(ctx)
	if err != nil {
		return err
	}

	newGlobals := make([]*net.IPNet, 0, len(instanceCIDRs))
	for _, cidr := range instanceCIDRs {
		_, ipNet, parseErr := NormalizeCIDR(cidr)
		if parseErr != nil {
			slog.Warn("Skipping invalid instance exclusion CIDR", "cidr", cidr, "error", parseErr)
			continue
		}
		newGlobals = append(newGlobals, ipNet)
	}

	newSites := make(map[uuid.UUID][]*net.IPNet)
	for _, rule := range siteCIDRs {
		_, ipNet, parseErr := NormalizeCIDR(rule.CIDR)
		if parseErr != nil {
			slog.Warn("Skipping invalid site exclusion CIDR", "site_id", rule.SiteID, "cidr", rule.CIDR, "error", parseErr)
			continue
		}
		newSites[rule.SiteID] = append(newSites[rule.SiteID], ipNet)
	}

	f.mu.Lock()
	f.globalRules = newGlobals
	f.siteRules = newSites
	f.mu.Unlock()

	return nil
}

func (f *IPFilter) IsBlocked(siteID uuid.UUID, ipStr string) bool {
	ip := parseIP(ipStr)
	if ip == nil {
		return false
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, blockedNetwork := range f.globalRules {
		if blockedNetwork.Contains(ip) {
			return true
		}
	}

	if siteNetworks, ok := f.siteRules[siteID]; ok {
		for _, blockedNetwork := range siteNetworks {
			if blockedNetwork.Contains(ip) {
				return true
			}
		}
	}

	return false
}

func parseIP(value string) net.IP {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		trimmed = host
	}

	return net.ParseIP(trimmed)
}
