package newsletter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncSettingsValidatesSchedule(t *testing.T) {
	store := &Store{}

	require.EqualError(t, store.SyncSettings(context.Background(), Settings{SendHour: 8}), "newsletter timezone is required")
	require.EqualError(t, store.SyncSettings(context.Background(), Settings{Timezone: "Asia/Colombo", SendHour: 24}), "newsletter send hour must be between 0 and 23")
}

func TestNormalizeSubscriberInput(t *testing.T) {
	input, err := normalizeSubscriberInput(SubscriberInput{
		Email:            "  Reader@Example.com ",
		Name:             "  Nipun  ",
		ConsentConfirmed: true,
	}, true)

	require.NoError(t, err)
	require.Equal(t, "reader@example.com", input.Email)
	require.Equal(t, "Nipun", input.Name)
	require.Equal(t, StatusActive, input.Status)
}

func TestNormalizeSubscriberInputRequiresConsent(t *testing.T) {
	_, err := normalizeSubscriberInput(SubscriberInput{Email: "reader@example.com"}, true)

	require.ErrorIs(t, err, ErrConsentRequired)
}

func TestNormalizeSubscriberInputRejectsDisplayAddress(t *testing.T) {
	_, err := normalizeSubscriberInput(SubscriberInput{
		Email:            "Reader <reader@example.com>",
		ConsentConfirmed: true,
	}, true)

	require.ErrorIs(t, err, ErrInvalidEmail)
}

func TestNormalizeSubscriberInputRejectsInvalidStatus(t *testing.T) {
	_, err := normalizeSubscriberInput(SubscriberInput{
		Email:            "reader@example.com",
		Status:           "deleted",
		ConsentConfirmed: true,
	}, true)

	require.ErrorIs(t, err, ErrInvalidStatus)
}

func TestStatusTransitionRequiresRenewedConsentAfterUnsubscribe(t *testing.T) {
	require.ErrorIs(t, validateStatusTransition(StatusUnsubscribed, SubscriberInput{Status: StatusActive}), ErrConsentRequired)
	require.NoError(t, validateStatusTransition(StatusUnsubscribed, SubscriberInput{Status: StatusActive, ConsentConfirmed: true}))
	require.NoError(t, validateStatusTransition(StatusPaused, SubscriberInput{Status: StatusActive}))
}
