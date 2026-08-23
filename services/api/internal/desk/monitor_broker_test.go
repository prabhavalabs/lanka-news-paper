package desk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonitorBrokerBroadcastsToSubscriber(t *testing.T) {
	broker := &MonitorBroker{subscribers: make(map[chan struct{}]struct{})}
	events, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	broker.broadcast()

	select {
	case <-events:
	default:
		require.Fail(t, "subscriber did not receive monitor change")
	}
}

func TestMonitorBrokerUnsubscribeStopsBroadcasts(t *testing.T) {
	broker := &MonitorBroker{subscribers: make(map[chan struct{}]struct{})}
	events, unsubscribe := broker.Subscribe()
	unsubscribe()

	broker.broadcast()

	select {
	case <-events:
		require.Fail(t, "unsubscribed listener received monitor change")
	default:
	}
}

func TestMonitorBrokerCoalescesPendingBroadcasts(t *testing.T) {
	broker := &MonitorBroker{subscribers: make(map[chan struct{}]struct{})}
	events, unsubscribe := broker.Subscribe()
	defer unsubscribe()

	broker.broadcast()
	broker.broadcast()

	select {
	case <-events:
	default:
		require.Fail(t, "subscriber did not receive monitor change")
	}
	select {
	case <-events:
		require.Fail(t, "duplicate pending monitor change was not coalesced")
	default:
	}
}
