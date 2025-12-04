package ingest

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nsqio/go-nsq"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/hklog"
)

type Consumer struct {
	store         *database.Store
	hitsConsumer  *nsq.Consumer
	eventConsumer *nsq.Consumer
	logger        *slog.Logger
	logLevel      slog.Level
}

func NewConsumer(store *database.Store, logger *slog.Logger, level slog.Level) *Consumer {
	return &Consumer{
		store:    store,
		logger:   logger,
		logLevel: level,
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
	var hit api.Hit
	if err := json.Unmarshal(m.Body, &hit); err != nil {
		slog.Error("Failed to unmarshal hit from NSQ", "error", err, "body", string(m.Body))
		return nil // Don't requeue malformed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.store.CreateHit(ctx, &hit); err != nil {
		slog.Error("Failed to create hit in database", "error", err, "site_id", hit.SiteID)
		return err
	}

	slog.Debug("Successfully processed hit", "site_id", hit.SiteID, "path", hit.Path)
	return nil
}

func (c *Consumer) handleEvent(m *nsq.Message) error {
	var event api.Event
	if err := json.Unmarshal(m.Body, &event); err != nil {
		slog.Error("Failed to unmarshal event from NSQ", "error", err, "body", string(m.Body))
		return nil // Don't requeue malformed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.store.CreateEvent(ctx, &event); err != nil {
		slog.Error("Failed to create event in database", "error", err, "site_id", event.SiteID)
		return err
	}

	slog.Debug("Successfully processed event", "site_id", event.SiteID, "name", event.Name)
	return nil
}
