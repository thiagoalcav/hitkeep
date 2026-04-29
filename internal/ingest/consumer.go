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
	hitBatcher    *storeBatcher[*api.Hit]
	eventBatcher  *storeBatcher[*api.Event]
	logger        *slog.Logger
	logLevel      slog.Level
}

func NewConsumer(tenantMgr *database.TenantStoreManager, logger *slog.Logger, level slog.Level) *Consumer {
	consumer := &Consumer{
		tenantMgr: tenantMgr,
		logger:    logger,
		logLevel:  level,
	}
	consumer.hitBatcher = newStoreBatcher("hit", logger, ingestBatchSize, ingestBatchFlushInterval, ingestPersistTimeout, func(store *database.Store, ctx context.Context, hits []*api.Hit) error {
		if err := store.CreateHitsBulk(ctx, hits); err != nil {
			return err
		}
		if err := tenantMgr.Shared().RecordHitActivity(ctx, hits); err != nil {
			logger.Warn("Failed to record hit activity summary after tenant persistence", "count", len(hits), "error", err)
		}
		return nil
	})
	consumer.eventBatcher = newStoreBatcher("event", logger, ingestBatchSize, ingestBatchFlushInterval, ingestPersistTimeout, func(store *database.Store, ctx context.Context, events []*api.Event) error {
		if err := store.CreateEventsBulk(ctx, events); err != nil {
			return err
		}
		if err := tenantMgr.Shared().RecordEventActivity(ctx, events); err != nil {
			logger.Warn("Failed to record event activity summary after tenant persistence", "count", len(events), "error", err)
		}
		return nil
	})
	return consumer
}

func (c *Consumer) Connect(addr string) error {
	// Hits Consumer
	hitsConfig := nsq.NewConfig()
	hitsConfig.MaxInFlight = ingestConsumerConcurrency
	hitsConsumer, err := nsq.NewConsumer("hits", "db-writer", hitsConfig)
	if err != nil {
		return err
	}
	hitsConsumer.SetLogger(hklog.GoNSQLogger{Logger: c.logger}, hklog.NSQGoLevel(c.logLevel))
	hitsConsumer.AddConcurrentHandlers(nsq.HandlerFunc(c.handleHit), ingestConsumerConcurrency)
	if err := hitsConsumer.ConnectToNSQD(addr); err != nil {
		return err
	}
	c.hitsConsumer = hitsConsumer

	// Events Consumer
	eventConfig := nsq.NewConfig()
	eventConfig.MaxInFlight = ingestConsumerConcurrency
	eventConsumer, err := nsq.NewConsumer("events", "db-writer", eventConfig)
	if err != nil {
		return err
	}
	eventConsumer.SetLogger(hklog.GoNSQLogger{Logger: c.logger}, hklog.NSQGoLevel(c.logLevel))
	eventConsumer.AddConcurrentHandlers(nsq.HandlerFunc(c.handleEvent), ingestConsumerConcurrency)
	if err := eventConsumer.ConnectToNSQD(addr); err != nil {
		return err
	}
	c.eventConsumer = eventConsumer

	return nil
}

func (c *Consumer) Stop() {
	if c.hitsConsumer != nil {
		c.hitsConsumer.Stop()
		<-c.hitsConsumer.StopChan
	}
	if c.eventConsumer != nil {
		c.eventConsumer.Stop()
		<-c.eventConsumer.StopChan
	}
	if c.hitBatcher != nil {
		c.hitBatcher.Stop()
	}
	if c.eventBatcher != nil {
		c.eventBatcher.Stop()
	}
}

func (c *Consumer) handleHit(m *nsq.Message) error {
	return processMessage(m, c, c.hitBatcher, func(v *api.Hit) (uuid.UUID, []any) {
		return v.SiteID, []any{"path", v.Path}
	}, "hit")
}

func (c *Consumer) handleEvent(m *nsq.Message) error {
	return processMessage(m, c, c.eventBatcher, func(v *api.Event) (uuid.UUID, []any) {
		return v.SiteID, []any{"name", v.Name}
	}, "event")
}

type siteIdentifiable interface {
	api.Hit | api.Event
}

func processMessage[T siteIdentifiable](
	m *nsq.Message,
	c *Consumer,
	batcher *storeBatcher[*T],
	identify func(*T) (uuid.UUID, []any),
	kind string,
) error {
	m.DisableAutoResponse()

	var v T
	if err := json.Unmarshal(m.Body, &v); err != nil {
		slog.Error("Failed to unmarshal "+kind+" from NSQ", "error", err, "body", string(m.Body))
		m.Finish()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	siteID, logAttrs := identify(&v)

	store, err := c.resolveStore(ctx, siteID)
	if err != nil {
		slog.Error("Failed to resolve tenant store for "+kind, "error", err, "site_id", siteID)
		m.Requeue(-1)
		return nil
	}

	result, err := batcher.Enqueue(batchItem[*T]{
		message:  m,
		value:    &v,
		store:    store,
		siteID:   siteID,
		logAttrs: logAttrs,
	})
	if err != nil {
		slog.Error("Failed to enqueue "+kind+" for batched persistence", "error", err, "site_id", siteID)
		m.Requeue(-1)
		return nil
	}

	if err := <-result; err != nil {
		slog.Error("Failed to persist "+kind+" batch", "error", err, "site_id", siteID)
		m.Requeue(-1)
		return nil
	}

	m.Finish()
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
