package performance

import (
	"context"
	"sync"
	"time"
	
	"github.com/tabular/local-pipeline/internal/logging"
	"github.com/tabular/local-pipeline/internal/storage"
)

type BatchProcessor struct {
	logger       *logging.Logger
	batchSize    int
	flushTimeout time.Duration
	queue        chan *storage.SpatialEvent
	batch        []*storage.SpatialEvent
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	processor    func([]*storage.SpatialEvent) error
}

func NewBatchProcessor(logger *logging.Logger, batchSize int, flushTimeout time.Duration, processor func([]*storage.SpatialEvent) error) *BatchProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	
	bp := &BatchProcessor{
		logger:       logger,
		batchSize:    batchSize,
		flushTimeout: flushTimeout,
		queue:        make(chan *storage.SpatialEvent, batchSize*2),
		batch:        make([]*storage.SpatialEvent, 0, batchSize),
		ctx:          ctx,
		cancel:       cancel,
		processor:    processor,
	}
	
	bp.wg.Add(1)
	go bp.run()
	
	return bp
}

func (bp *BatchProcessor) Add(event *storage.SpatialEvent) {
	select {
	case bp.queue <- event:
	case <-bp.ctx.Done():
		bp.logger.Warn("Batch processor stopped, dropping event", "event_id", event.EventID)
	}
}

func (bp *BatchProcessor) run() {
	defer bp.wg.Done()
	
	ticker := time.NewTicker(bp.flushTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case event := <-bp.queue:
			bp.mu.Lock()
			bp.batch = append(bp.batch, event)
			shouldFlush := len(bp.batch) >= bp.batchSize
			bp.mu.Unlock()
			
			if shouldFlush {
				bp.flush()
			}
			
		case <-ticker.C:
			bp.flush()
			
		case <-bp.ctx.Done():
			bp.flush() // Final flush
			return
		}
	}
}

func (bp *BatchProcessor) flush() {
	bp.mu.Lock()
	if len(bp.batch) == 0 {
		bp.mu.Unlock()
		return
	}
	
	toProcess := make([]*storage.SpatialEvent, len(bp.batch))
	copy(toProcess, bp.batch)
	bp.batch = bp.batch[:0]
	bp.mu.Unlock()
	
	startTime := time.Now()
	err := bp.processor(toProcess)
	duration := time.Since(startTime)
	
	if err != nil {
		bp.logger.Error("Batch processing failed", 
			"batch_size", len(toProcess),
			"duration", duration,
			"error", err)
	} else {
		bp.logger.Debug("Batch processed successfully",
			"batch_size", len(toProcess),
			"duration", duration)
	}
}

func (bp *BatchProcessor) Stop() {
	bp.cancel()
	bp.wg.Wait()
	close(bp.queue)
}