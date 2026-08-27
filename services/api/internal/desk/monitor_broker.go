package desk

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	queueMonitorChannel       = "queue_monitor_changed"
	monitorHealthInterval     = 15 * time.Second
	monitorListenerRetryDelay = time.Second
	monitorQuietWindow        = time.Second
	monitorMaximumDelay       = 5 * time.Second
)

// MonitorBroker fans database change notifications out to every active SSE stream.
// It holds one PostgreSQL LISTEN connection regardless of the number of browsers.
type MonitorBroker struct {
	pool        *pgxpool.Pool
	logger      *slog.Logger
	mu          sync.Mutex
	subscribers map[chan struct{}]struct{}
	changes     chan struct{}
}

// NewMonitorBroker starts the shared queue-monitor notification listener.
func NewMonitorBroker(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) *MonitorBroker {
	if logger == nil {
		logger = slog.Default()
	}
	broker := &MonitorBroker{
		pool:        pool,
		logger:      logger,
		subscribers: make(map[chan struct{}]struct{}),
		changes:     make(chan struct{}, 1),
	}
	go broker.listen(ctx)
	go broker.publishHealthChecks(ctx)
	go broker.dispatchChanges(ctx)
	return broker
}

// Subscribe registers a coalescing change channel and its idempotent cleanup function.
func (broker *MonitorBroker) Subscribe() (<-chan struct{}, func()) {
	events := make(chan struct{}, 1)
	broker.mu.Lock()
	broker.subscribers[events] = struct{}{}
	broker.mu.Unlock()

	var once sync.Once
	return events, func() {
		once.Do(func() {
			broker.mu.Lock()
			delete(broker.subscribers, events)
			broker.mu.Unlock()
		})
	}
}

func (broker *MonitorBroker) broadcast() {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	for subscriber := range broker.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func (broker *MonitorBroker) signalChange() {
	select {
	case broker.changes <- struct{}{}:
	default:
	}
}

func (broker *MonitorBroker) dispatchChanges(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-broker.changes:
		}

		quietTimer := time.NewTimer(monitorQuietWindow)
		maximumTimer := time.NewTimer(monitorMaximumDelay)
	collect:
		for {
			select {
			case <-ctx.Done():
				quietTimer.Stop()
				maximumTimer.Stop()
				return
			case <-broker.changes:
				resetMonitorTimer(quietTimer, monitorQuietWindow)
			case <-quietTimer.C:
				maximumTimer.Stop()
				broker.broadcast()
				break collect
			case <-maximumTimer.C:
				quietTimer.Stop()
				broker.broadcast()
				break collect
			}
		}
	}
}

func resetMonitorTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func (broker *MonitorBroker) publishHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(monitorHealthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			broker.signalChange()
		}
	}
}

func (broker *MonitorBroker) listen(ctx context.Context) {
	for ctx.Err() == nil {
		if err := broker.listenUntilError(ctx); err != nil && ctx.Err() == nil {
			broker.logger.WarnContext(ctx, "queue monitor listener disconnected", "error", err)
		}
		if !waitForMonitorRetry(ctx) {
			return
		}
	}
}

func (broker *MonitorBroker) listenUntilError(ctx context.Context) error {
	connection, err := broker.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer broker.releaseListener(connection)

	if _, err := connection.Exec(ctx, "LISTEN "+queueMonitorChannel); err != nil {
		return err
	}
	for {
		if _, err := connection.Conn().WaitForNotification(ctx); err != nil {
			return err
		}
		broker.signalChange()
	}
}

func (broker *MonitorBroker) releaseListener(connection *pgxpool.Conn) {
	cleanupContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := connection.Exec(cleanupContext, "UNLISTEN "+queueMonitorChannel); err != nil {
		broker.logger.Debug("could not reset queue monitor listener", "error", err)
		if closeErr := connection.Conn().Close(cleanupContext); closeErr != nil {
			broker.logger.Debug("could not close queue monitor listener", "error", closeErr)
		}
	}
	connection.Release()
}

func waitForMonitorRetry(ctx context.Context) bool {
	timer := time.NewTimer(monitorListenerRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
