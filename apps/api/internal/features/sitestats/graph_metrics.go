package sitestats

import (
	"context"
	"sync"
	"time"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type graphMetricBatch struct {
	ctx        context.Context
	increments map[string]int64
	done       chan struct{}
}

// RecordGraph combines concurrent displays into synchronous database batches. Each
// caller waits for its batch; batches never compete for the same row locks.
func (service *Service) RecordGraph(ctx context.Context, ids []string) {
	if len(ids) == 0 || ctx.Err() != nil {
		return
	}
	service.graphMu.Lock()
	if service.graphClosed {
		service.graphMu.Unlock()
		return
	}
	batch := service.graphPending
	if batch == nil {
		batch = &graphMetricBatch{ctx: context.WithoutCancel(ctx), increments: make(map[string]int64, len(ids)), done: make(chan struct{})}
		service.graphPending = batch
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; !exists {
			batch.increments[id]++
			seen[id] = struct{}{}
		}
	}
	if !service.graphWriting {
		service.graphWriting = true
		service.graphWorkers.Add(1)
		go service.flushGraphMetrics()
	}
	service.graphMu.Unlock()
	// Once accepted, wait even if the reader disconnects. HTTP shutdown therefore
	// drains accepted writes before closing the database pool. At most one active
	// batch precedes this one; each database write has a one-second deadline.
	<-batch.done
}

func (service *Service) flushGraphMetrics() {
	defer service.graphWorkers.Done()
	for {
		timer := time.NewTimer(5 * time.Millisecond)
		<-timer.C
		service.graphMu.Lock()
		batch := service.graphPending
		if batch == nil {
			service.graphWriting = false
			service.graphMu.Unlock()
			return
		}
		service.graphPending = nil
		service.graphMu.Unlock()
		parameters := dbgen.IncrementGraphMetricsParams{
			ShortIds:   make([]string, 0, len(batch.increments)),
			Increments: make([]int64, 0, len(batch.increments)),
		}
		for id, count := range batch.increments {
			parameters.ShortIds = append(parameters.ShortIds, id)
			parameters.Increments = append(parameters.Increments, count)
		}
		// Accepted events share a bounded write; one cancelled reader cannot cancel
		// the others. The writer exits as soon as the request backlog is empty.
		bounded, cancel := context.WithTimeout(batch.ctx, time.Second)
		service.writeGraphBatch(bounded, parameters)
		cancel()
		close(batch.done)
	}
}

func (service *Service) writeGraphBatch(ctx context.Context, parameters dbgen.IncrementGraphMetricsParams) {
	// Large graphs use at most eight pool connections. Each ID belongs to exactly
	// one partition, and the next batch waits for all partitions to finish. Small
	// neighborhood graphs retain one write. Each site's two counters stay atomic.
	partitions := min(8, max(1, len(parameters.ShortIds)/1024))
	width := (len(parameters.ShortIds) + partitions - 1) / partitions
	var writes sync.WaitGroup
	for start := 0; start < len(parameters.ShortIds); start += width {
		end := min(start+width, len(parameters.ShortIds))
		part := dbgen.IncrementGraphMetricsParams{ShortIds: parameters.ShortIds[start:end], Increments: parameters.Increments[start:end]}
		writes.Go(func() {
			if err := service.store.IncrementGraphMetrics(ctx, part); err != nil {
				service.report(ctx, err, "graph")
			}
		})
	}
	writes.Wait()
}

// Close stops accepting graph counts and drains the bounded writes before the
// owner closes its database pool. It is safe to call concurrently or repeatedly.
func (service *Service) Close() {
	service.graphMu.Lock()
	service.graphClosed = true
	service.graphMu.Unlock()
	service.graphWorkers.Wait()
}
