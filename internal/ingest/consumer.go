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
	store    *database.Store
	consumer *nsq.Consumer
	logger   *slog.Logger
	logLevel slog.Level
}

func NewConsumer(store *database.Store, logger *slog.Logger, level slog.Level) *Consumer {
	return &Consumer{
		store:    store,
		logger:   logger,
		logLevel: level,
	}
}

func (c *Consumer) Connect(addr string) error {
	consumer, err := nsq.NewConsumer("hits", "db-writer", nsq.NewConfig())
	if err != nil {
		return err
	}

	// Wire up Consumer logger to slog
	consumer.SetLogger(hklog.GoNSQLogger{Logger: c.logger}, hklog.NSQGoLevel(c.logLevel))

	consumer.AddHandler(c) // The consumer itself is the handler
	if err := consumer.ConnectToNSQD(addr); err != nil {
		return err
	}
	c.consumer = consumer
	return nil
}

func (c *Consumer) Stop() {
	c.consumer.Stop()
}

// HandleMessage implements the nsq.Handler interface.
// This is called for every message received from the "hits" topic.
func (c *Consumer) HandleMessage(m *nsq.Message) error {
	var hit api.Hit
	if err := json.Unmarshal(m.Body, &hit); err != nil {
		slog.Error("Failed to unmarshal hit from NSQ", "error", err, "body", string(m.Body))
		// Do not requeue malformed messages.
		return nil
	}

	// Create a context with a timeout for the database operation.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call the CreateHit method on the store.
	if err := c.store.CreateHit(ctx, &hit); err != nil {
		slog.Error("Failed to create hit in database", "error", err, "site_id", hit.SiteID)
		// Return the error to NSQ so it will be retried.
		return err
	}

	slog.Debug("Successfully processed hit", "site_id", hit.SiteID, "path", hit.Path)
	return nil
}
