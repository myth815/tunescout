package provider

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

type audius struct {
	client  *http.Client
	baseURL string
}

func NewAudius(client *http.Client, cfg config.Config) Provider {
	return &audius{client: client, baseURL: cfg.AudiusBaseURL}
}

func (p *audius) Info() model.ProviderInfo {
	return model.ProviderInfo{ID: "audius", Enabled: true, Configured: true, Capabilities: []string{"search:recording", "lookup", "offers:source"}}
}

func (p *audius) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if !wantsType(request.Types, "recording", "release_track") || strings.TrimSpace(request.Query) == "" {
		return nil, nil
	}
	values := url.Values{"query": {request.Query}}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/tracks/search?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Duration  int64  `json:"duration"`
			Genre     string `json:"genre"`
			Permalink string `json:"permalink"`
			Artwork   struct {
				Large  string `json:"1000x1000"`
				Medium string `json:"480x480"`
			} `json:"artwork"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := getJSON(p.client, httpRequest, &response); err != nil {
		return nil, err
	}
	limit := providerLimit(request.Limit)
	if len(response.Data) > limit {
		response.Data = response.Data[:limit]
	}
	hits := make([]model.Hit, 0, len(response.Data))
	for _, item := range response.Data {
		score := textScore(request.Query, item.Title+" "+item.User.Name)
		artwork := item.Artwork.Large
		if artwork == "" {
			artwork = item.Artwork.Medium
		}
		refURL := item.Permalink
		if refURL != "" && !strings.HasPrefix(refURL, "http") {
			refURL = "https://audius.co" + refURL
		}
		hits = append(hits, model.Hit{
			EntityType:        "recording",
			Summary:           model.Summary{Title: item.Title, Artists: nonEmpty(item.User.Name), DurationMS: item.Duration * 1000, ArtworkURL: artwork},
			RankScore:         score,
			Match:             model.Match{Quality: textQuality(score), Fields: []string{"title", "artist"}, Reasons: []string{"Audius independent-music search"}},
			Providers:         []string{"audius"},
			ProviderRefs:      []model.ProviderRef{{Provider: "audius", Type: "recording", ID: item.ID, URL: refURL}},
			AvailableSections: []string{"artwork", "offers"},
			Offers:            []model.Offer{{Type: "source_page", Provider: "audius", Access: "free", URL: refURL, Rights: model.OfferRights{}}},
		})
	}
	return hits, nil
}

func (p *audius) Lookup(ctx context.Context, ref model.ProviderRef, include []string) (map[string]any, error) {
	if ref.Provider != "audius" {
		return nil, ErrUnsupported
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/tracks/"+url.PathEscape(ref.ID), nil)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := getJSON(p.client, request, &response); err != nil {
		return nil, err
	}
	return response, nil
}
