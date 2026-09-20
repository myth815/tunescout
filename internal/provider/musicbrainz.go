package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

type musicBrainz struct {
	client    *http.Client
	baseURL   string
	userAgent string
	rateMu    sync.Mutex
	lastCall  time.Time
}

type artistCredit struct {
	Name string `json:"name"`
}

func NewMusicBrainz(client *http.Client, cfg config.Config) Provider {
	return &musicBrainz{client: client, baseURL: cfg.MusicBrainzBaseURL, userAgent: cfg.UserAgent}
}

func (p *musicBrainz) Info() model.ProviderInfo {
	return model.ProviderInfo{ID: "musicbrainz", Enabled: true, Configured: true, Capabilities: []string{"search:artist", "search:recording", "search:release", "lookup"}}
}

func (p *musicBrainz) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if strings.TrimSpace(request.Query) == "" {
		return nil, nil
	}
	var hits []model.Hit
	var firstErr error
	if wantsType(request.Types, "artist") {
		items, err := p.searchArtists(ctx, request.Query, request.Limit)
		if err != nil {
			firstErr = err
		} else {
			hits = append(hits, items...)
		}
	}
	if wantsType(request.Types, "recording", "release_track") {
		items, err := p.searchRecordings(ctx, request.Query, request.Limit)
		if err != nil && firstErr == nil {
			firstErr = err
		} else {
			hits = append(hits, items...)
		}
	}
	if wantsType(request.Types, "release") {
		items, err := p.searchReleases(ctx, request.Query, request.Limit)
		if err != nil && firstErr == nil {
			firstErr = err
		} else {
			hits = append(hits, items...)
		}
	}
	return hits, firstErr
}

func (p *musicBrainz) searchArtists(ctx context.Context, query string, limit int) ([]model.Hit, error) {
	var response struct {
		Artists []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			SortName       string `json:"sort-name"`
			Score          int    `json:"score"`
			Country        string `json:"country"`
			Disambiguation string `json:"disambiguation"`
			Aliases        []struct {
				Name string `json:"name"`
			} `json:"aliases"`
		} `json:"artists"`
	}
	if err := p.search(ctx, "artist", query, limit, &response); err != nil {
		return nil, err
	}
	hits := make([]model.Hit, 0, len(response.Artists))
	for _, item := range response.Artists {
		aliases := make([]string, 0, len(item.Aliases))
		for _, alias := range item.Aliases {
			if alias.Name != "" {
				aliases = append(aliases, alias.Name)
			}
		}
		hits = append(hits, model.Hit{
			EntityType:        "artist",
			Summary:           model.Summary{Name: item.Name, SortName: item.SortName, Aliases: aliases, Country: item.Country, Disambiguation: item.Disambiguation},
			RankScore:         float64(item.Score) / 100,
			Match:             model.Match{Quality: quality(item.Score), Fields: []string{"name", "alias"}, Reasons: []string{"MusicBrainz catalog search"}},
			Providers:         []string{"musicbrainz"},
			ProviderRefs:      []model.ProviderRef{{Provider: "musicbrainz", Type: "artist", ID: item.ID, URL: "https://musicbrainz.org/artist/" + item.ID}},
			AvailableSections: []string{"recordings", "releases", "external_ids"},
		})
	}
	return hits, nil
}

func (p *musicBrainz) searchRecordings(ctx context.Context, query string, limit int) ([]model.Hit, error) {
	var response struct {
		Recordings []struct {
			ID           string         `json:"id"`
			Title        string         `json:"title"`
			Score        int            `json:"score"`
			Length       int64          `json:"length"`
			ISRCs        []string       `json:"isrcs"`
			ArtistCredit []artistCredit `json:"artist-credit"`
			Releases     []struct {
				Title string `json:"title"`
				Date  string `json:"date"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	if err := p.search(ctx, "recording", query, limit, &response); err != nil {
		return nil, err
	}
	hits := make([]model.Hit, 0, len(response.Recordings))
	for _, item := range response.Recordings {
		artists := creditNames(item.ArtistCredit)
		summary := model.Summary{Title: item.Title, Artists: artists, DurationMS: item.Length}
		if len(item.Releases) > 0 {
			summary.PrimaryRelease = item.Releases[0].Title
			summary.Date = item.Releases[0].Date
		}
		external := map[string]any{"musicbrainz_recording_id": item.ID}
		if len(item.ISRCs) > 0 {
			external["isrc"] = item.ISRCs
		}
		hits = append(hits, model.Hit{
			EntityType: "recording", Summary: summary, RankScore: float64(item.Score) / 100,
			Match:     model.Match{Quality: quality(item.Score), Fields: []string{"title", "artist", "release"}, Reasons: []string{"MusicBrainz catalog search"}},
			Providers: []string{"musicbrainz"}, ExternalIDs: external,
			ProviderRefs:      []model.ProviderRef{{Provider: "musicbrainz", Type: "recording", ID: item.ID, URL: "https://musicbrainz.org/recording/" + item.ID}},
			AvailableSections: []string{"releases", "credits", "external_ids"},
		})
	}
	return hits, nil
}

func (p *musicBrainz) searchReleases(ctx context.Context, query string, limit int) ([]model.Hit, error) {
	var response struct {
		Releases []struct {
			ID           string         `json:"id"`
			Title        string         `json:"title"`
			Score        int            `json:"score"`
			Date         string         `json:"date"`
			Country      string         `json:"country"`
			Barcode      string         `json:"barcode"`
			ArtistCredit []artistCredit `json:"artist-credit"`
		} `json:"releases"`
	}
	if err := p.search(ctx, "release", query, limit, &response); err != nil {
		return nil, err
	}
	hits := make([]model.Hit, 0, len(response.Releases))
	for _, item := range response.Releases {
		external := map[string]any{"musicbrainz_release_id": item.ID}
		if item.Barcode != "" {
			external["barcode"] = item.Barcode
		}
		hits = append(hits, model.Hit{
			EntityType: "release", Summary: model.Summary{Title: item.Title, Artists: creditNames(item.ArtistCredit), Date: item.Date, Country: item.Country},
			RankScore: float64(item.Score) / 100,
			Match:     model.Match{Quality: quality(item.Score), Fields: []string{"title", "artist"}, Reasons: []string{"MusicBrainz catalog search"}},
			Providers: []string{"musicbrainz"}, ExternalIDs: external,
			ProviderRefs:      []model.ProviderRef{{Provider: "musicbrainz", Type: "release", ID: item.ID, URL: "https://musicbrainz.org/release/" + item.ID}},
			AvailableSections: []string{"tracks", "credits", "artwork", "external_ids"},
		})
	}
	return hits, nil
}

func (p *musicBrainz) search(ctx context.Context, resource, query string, limit int, target any) error {
	values := url.Values{"query": {query}, "fmt": {"json"}, "limit": {strconv.Itoa(providerLimit(limit))}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/ws/2/%s/?%s", p.baseURL, resource, values.Encode()), nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", p.userAgent)
	request.Header.Set("Accept", "application/json")
	return p.getJSON(ctx, request, target)
}

func (p *musicBrainz) Lookup(ctx context.Context, ref model.ProviderRef, include []string) (map[string]any, error) {
	if ref.Provider != "musicbrainz" {
		return nil, ErrUnsupported
	}
	inc := map[string]string{
		"artist":    "aliases+url-rels+recordings+releases",
		"recording": "artists+releases+isrcs+work-rels+url-rels",
		"release":   "artists+recordings+labels+release-groups+media+url-rels",
	}[ref.Type]
	if inc == "" {
		return nil, ErrUnsupported
	}
	values := url.Values{"fmt": {"json"}, "inc": {inc}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/ws/2/%s/%s?%s", p.baseURL, ref.Type, url.PathEscape(ref.ID), values.Encode()), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", p.userAgent)
	request.Header.Set("Accept", "application/json")
	var response map[string]any
	if err := p.getJSON(ctx, request, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (p *musicBrainz) getJSON(ctx context.Context, request *http.Request, target any) error {
	p.rateMu.Lock()
	defer p.rateMu.Unlock()
	if !p.lastCall.IsZero() {
		if wait := time.Second - time.Since(p.lastCall); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	p.lastCall = time.Now()
	return getJSON(p.client, request, target)
}

func creditNames(credits []artistCredit) []string {
	names := make([]string, 0, len(credits))
	for _, credit := range credits {
		if strings.TrimSpace(credit.Name) != "" {
			names = append(names, credit.Name)
		}
	}
	return names
}

func quality(score int) string {
	switch {
	case score >= 95:
		return "exact"
	case score >= 80:
		return "strong"
	case score >= 60:
		return "fuzzy"
	default:
		return "related"
	}
}

func providerLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 25 {
		return 25
	}
	return limit
}
