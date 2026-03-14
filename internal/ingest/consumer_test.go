package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

func testBatchLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestStoreBatcherFlushesByStore(t *testing.T) {
	storeA := &database.Store{}
	storeB := &database.Store{}

	var mu sync.Mutex
	flushed := make(map[*database.Store]int)

	batcher := newStoreBatcher("hit", testBatchLogger(), 3, time.Hour, time.Second, func(store *database.Store, _ context.Context, hits []*api.Hit) error {
		mu.Lock()
		defer mu.Unlock()
		flushed[store] += len(hits)
		return nil
	})
	defer batcher.Stop()

	results := make([]<-chan error, 0, 3)
	for _, item := range []batchItem[*api.Hit]{
		{store: storeA, siteID: uuid.New(), value: &api.Hit{Path: "/pricing"}},
		{store: storeA, siteID: uuid.New(), value: &api.Hit{Path: "/signup"}},
		{store: storeB, siteID: uuid.New(), value: &api.Hit{Path: "/docs"}},
	} {
		result, err := batcher.Enqueue(item)
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		results = append(results, result)
	}

	for _, result := range results {
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("unexpected batch error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for batched flush")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if flushed[storeA] != 2 {
		t.Fatalf("expected storeA to flush 2 hits, got %d", flushed[storeA])
	}
	if flushed[storeB] != 1 {
		t.Fatalf("expected storeB to flush 1 hit, got %d", flushed[storeB])
	}
}

func TestStoreBatcherFlushesOnIntervalAndPropagatesError(t *testing.T) {
	expectedErr := errors.New("boom")
	store := &database.Store{}

	batcher := newStoreBatcher("event", testBatchLogger(), 10, 10*time.Millisecond, time.Second, func(_ *database.Store, _ context.Context, _ []*api.Event) error {
		return expectedErr
	})
	defer batcher.Stop()

	result, err := batcher.Enqueue(batchItem[*api.Event]{
		store:  store,
		siteID: uuid.New(),
		value:  &api.Event{Name: "signup"},
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case gotErr := <-result:
		if !errors.Is(gotErr, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, gotErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for interval flush")
	}
}
