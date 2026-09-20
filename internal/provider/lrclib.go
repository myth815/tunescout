package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

type lrcLib struct {
	client  *http.Client
	baseURL string
}

type lrcLibTrack struct {
	ID           int64   `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func NewLRCLIB(client *http.Client, cfg config.Config) Provider {
	return &lrcLib{client: client, baseURL: cfg.LRCLIBBaseURL}
}

func (p *lrcLib) Info() model.ProviderInfo {
	return model.ProviderInfo{ID: "lrclib", Enabled: true, Configured: true, Capabilities: []string{"search:recording", "lookup", "lyrics"}}
}

func (p *lrcLib) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if !wantsType(request.Types, "recording", "release_track", "lyrics") || strings.TrimSpace(request.Query) == "" {
		return nil, nil
	}
	values := url.Values{"q": {request.Query}}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/search?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var tracks []lrcLibTrack
	if err := getJSON(p.client, httpRequest, &tracks); err != nil {
		return nil, err
	}
	limit := providerLimit(request.Limit)
	if len(tracks) > limit {
		tracks = tracks[:limit]
	}
	hits := make([]model.Hit, 0, len(tracks))
	for _, track := range tracks {
		score := textScore(request.Query, track.TrackName+" "+track.ArtistName+" "+track.AlbumName)
		sections := []string{"lyrics"}
		hits = append(hits, model.Hit{
			EntityType:        "recording",
			Summary:           model.Summary{Title: track.TrackName, Artists: nonEmpty(track.ArtistName), PrimaryRelease: track.AlbumName, DurationMS: int64(track.Duration * 1000)},
			RankScore:         score,
			Match:             model.Match{Quality: textQuality(score), Fields: []string{"title", "artist", "release"}, Reasons: []string{"LRCLIB metadata search"}},
			Providers:         []string{"lrclib"},
			ProviderRefs:      []model.ProviderRef{{Provider: "lrclib", Type: "recording", ID: strconv.FormatInt(track.ID, 10), URL: fmt.Sprintf("%s/api/get/%d", p.baseURL, track.ID)}},
			AvailableSections: sections,
		})
	}
	return hits, nil
}

func (p *lrcLib) Lookup(ctx context.Context, ref model.ProviderRef, include []string) (map[string]any, error) {
	if ref.Provider != "lrclib" {
		return nil, ErrUnsupported
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/get/"+url.PathEscape(ref.ID), nil)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := getJSON(p.client, request, &response); err != nil {
		return nil, err
	}
	return response, nil
}
