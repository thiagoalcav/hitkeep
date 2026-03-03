package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/hklog"
)

type Consumer struct {
	tenantMgr     *database.TenantStoreManager
	hitsConsumer  *nsq.Consumer
	eventConsumer *nsq.Consumer
	logger        *slog.Logger
	logLevel      slog.Level
}

func NewConsumer(tenantMgr *database.TenantStoreManager, logger *slog.Logger, level slog.Level) *Consumer {
	return &Consumer{
		tenantMgr: tenantMgr,
		logger:    logger,
		logLevel:  level,
	}
}

func (c *Consumer) Connect(addr string) error {
	// Hits Consumer
	hitsConsumer, err := nsq.NewConsumer("hits", "db-writer", nsq.NewConfig())
	if err != nil {
		return err
	}
	hitsConsumer.SetLogger(hklog.GoNSQLogger{Logger: c.logger}, hklog.NSQGoLevel(c.logLevel))
	hitsConsumer.AddHandler(nsq.HandlerFunc(c.handleHit))
	if err := hitsConsumer.ConnectToNSQD(addr); err != nil {
		return err
	}
	c.hitsConsumer = hitsConsumer

	// Events Consumer
	eventConsumer, err := nsq.NewConsumer("events", "db-writer", nsq.NewConfig())
	if err != nil {
		return err
	}
	eventConsumer.SetLogger(hklog.GoNSQLogger{Logger: c.logger}, hklog.NSQGoLevel(c.logLevel))
	eventConsumer.AddHandler(nsq.HandlerFunc(c.handleEvent))
	if err := eventConsumer.ConnectToNSQD(addr); err != nil {
		return err
	}
	c.eventConsumer = eventConsumer

	return nil
}

func (c *Consumer) Stop() {
	if c.hitsConsumer != nil {
		c.hitsConsumer.Stop()
	}
	if c.eventConsumer != nil {
		c.eventConsumer.Stop()
	}
}

func (c *Consumer) handleHit(m *nsq.Message) error {
	return processMessage(m, c, func(store *database.Store, ctx context.Context, hit *api.Hit) error {
		return store.CreateHit(ctx, hit)
	}, func(v *api.Hit) (uuid.UUID, []any) {
		return v.SiteID, []any{"path", v.Path}
	}, "hit")
}

func (c *Consumer) handleEvent(m *nsq.Message) error {
	return processMessage(m, c, func(store *database.Store, ctx context.Context, event *api.Event) error {
		return store.CreateEvent(ctx, event)
	}, func(v *api.Event) (uuid.UUID, []any) {
		return v.SiteID, []any{"name", v.Name}
	}, "event")
}

type siteIdentifiable interface {
	api.Hit | api.Event
}

func processMessage[T siteIdentifiable](
	m *nsq.Message,
	c *Consumer,
	persist func(*database.Store, context.Context, *T) error,
	identify func(*T) (uuid.UUID, []any),
	kind string,
) error {
	var v T
	if err := json.Unmarshal(m.Body, &v); err != nil {
		slog.Error("Failed to unmarshal "+kind+" from NSQ", "error", err, "body", string(m.Body))
		return nil // Don't requeue malformed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	siteID, logAttrs := identify(&v)

	store, err := c.resolveStore(ctx, siteID)
	if err != nil {
		slog.Error("Failed to resolve tenant store for "+kind, "error", err, "site_id", siteID)
		return err
	}

	if err := persist(store, ctx, &v); err != nil {
		slog.Error("Failed to create "+kind+" in database", "error", err, "site_id", siteID)
		return err
	}

	slog.Debug("Successfully processed "+kind, append([]any{"site_id", siteID}, logAttrs...)...)
	return nil
}

func (c *Consumer) resolveStore(ctx context.Context, siteID uuid.UUID) (*database.Store, error) {
	store, _, err := c.tenantMgr.ResolveSiteStore(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("resolve analytics store for site %s: %w", siteID, err)
	}
	return store, nil
}
