package social

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/loomcast"
)

type loomcastClient interface {
	Channels(context.Context) ([]loomcast.Channel, error)
	CreatePost(context.Context, string, loomcast.NewPost) (loomcast.Post, error)
	Upload(context.Context, loomcast.Upload, []byte) error
	Finalize(context.Context, string) (loomcast.Post, error)
	Post(context.Context, string) (loomcast.Post, error)
	Cancel(context.Context, string) (loomcast.Post, error)
}

// channelLookup is the Loomcast channel posts go to, and why it cannot publish now, if it cannot.
type channelLookup struct {
	channel *loomcast.Channel
	problem string
	warning string
	at      time.Time
}

func (lookup channelLookup) ready() bool { return lookup.channel != nil && lookup.problem == "" }

// The status card and publish plans read the channel often; the publisher always reads it fresh.
const channelCacheTTL = time.Minute

func (service *Service) publishChannel(ctx context.Context, fresh bool) channelLookup {
	if !service.config.Configured() {
		return channelLookup{problem: "Set SNAP_LOOMCAST_API_KEY on the server to publish through Loomcast."}
	}
	service.channelMu.Lock()
	defer service.channelMu.Unlock()
	if !fresh && !service.channelCache.at.IsZero() && time.Since(service.channelCache.at) < channelCacheTTL {
		return service.channelCache
	}
	lookupContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	channels, err := service.loomcast.Channels(lookupContext)
	result := resolveChannel(channels, err, service.config.LoomcastChannelID)
	result.at = time.Now()
	service.channelCache = result
	return result
}

func (service *Service) forgetChannel() {
	service.channelMu.Lock()
	service.channelCache = channelLookup{}
	service.channelMu.Unlock()
}

func resolveChannel(channels []loomcast.Channel, err error, wanted string) channelLookup {
	if err != nil {
		return channelLookup{problem: describeLoomcastError(err)}
	}
	matches := make([]loomcast.Channel, 0, 1)
	for _, channel := range channels {
		if channel.Platform == "facebook" && (wanted == "" || channel.ID == wanted) {
			matches = append(matches, channel)
		}
	}
	switch {
	case len(matches) == 0 && wanted != "":
		return channelLookup{problem: "The channel in SNAP_LOOMCAST_CHANNEL_ID is not a Facebook channel granted to this Loomcast API client."}
	case len(matches) == 0:
		return channelLookup{problem: "No Facebook channel is granted to this Loomcast API client. In Loomcast, open Settings → API clients and grant the Thulawa News Page."}
	case len(matches) > 1:
		return channelLookup{problem: "Several Facebook channels are granted to this Loomcast API client. Set SNAP_LOOMCAST_CHANNEL_ID to the Thulawa News Page's channel id."}
	}
	channel := matches[0]
	result := channelLookup{channel: &channel}
	switch {
	case channel.Status != "active":
		result.problem = "Loomcast reports the " + channel.Name + " channel as " + channel.Status + ". Reconnect it in Loomcast under Settings → Channels."
	case channel.Access == "read":
		result.problem = "The Loomcast API client may only read " + channel.Name + ". Give it publish access in Loomcast under Settings → API clients."
	case len(channel.Formats) > 0 && !slices.Contains(channel.Formats, "image"):
		result.problem = "Loomcast does not publish photo posts on " + channel.Name + "."
	case channel.DryRun:
		result.problem = channel.Name + " is in dry run in Loomcast, so nothing would reach Facebook. Turn dry run off for it under Settings → Channels."
	case channel.Access != "publish" || channel.RequiresApproval:
		result.warning = "Each post waits for the owner's approval in Loomcast. To post automatically, give the API client publish access and turn off Requires approval for " + channel.Name + "."
	}
	return result
}

func describeLoomcastError(err error) string {
	var apiError *loomcast.APIError
	var transport *loomcast.TransportError
	switch {
	case errors.As(err, &apiError) && apiError.Status == 401:
		return "Loomcast refused the API key. Create a new API client in Loomcast and update SNAP_LOOMCAST_API_KEY."
	case errors.As(err, &apiError) && apiError.Status == 403:
		return "Loomcast refused this API client: its owner is no longer the workspace owner. Create a new API client in Loomcast."
	case errors.As(err, &apiError) && apiError.Status == 429:
		return "Loomcast is rate limiting this API client; publishing resumes shortly."
	case errors.As(err, &apiError):
		return truncate("Loomcast could not list the channels: "+apiError.Error(), 500)
	case errors.As(err, &transport):
		return "Loomcast could not be reached; publishing resumes when it answers."
	}
	return "Loomcast could not be reached; publishing resumes when it answers."
}

func publishChannelOf(lookup channelLookup) *PublishChannel {
	if lookup.channel == nil {
		return nil
	}
	channel := lookup.channel
	return &PublishChannel{ID: channel.ID, Platform: channel.Platform, Name: channel.Name, Status: channel.Status,
		Access: channel.Access, DryRun: channel.DryRun, RequiresApproval: channel.RequiresApproval}
}

func (store *Store) publishRetryAt(ctx context.Context, reader historyReader) (*time.Time, error) {
	rows, err := reader.Query(ctx, `SELECT publish_retry_at FROM social_settings WHERE singleton`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var at *time.Time
	if rows.Next() {
		if err := rows.Scan(&at); err != nil {
			return nil, err
		}
	}
	return at, rows.Err()
}
