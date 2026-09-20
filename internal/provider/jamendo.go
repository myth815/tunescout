package provider

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

type jamendo struct {
	client   *http.Client
	baseURL  string
	clientID string
}

func NewJamendo(client *http.Client, cfg config.Config) Provider {
	return &jamendo{client: client, baseURL: cfg.JamendoBaseURL, clientID: cfg.JamendoClientID}
}

func (p *jamendo) Info() model.ProviderInfo {
	configured := p.clientID != ""
	return model.ProviderInfo{ID: "jamendo", Enabled: configured, Configured: configured, RequiresKey: true, Capabilities: []string{"search:recording", "lookup", "offers:stream", "offers:download"}}
}

func (p *jamendo) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if p.clientID == "" || !wantsType(request.Types, "recording", "release_track") || strings.TrimSpace(request.Query) == "" {
		return nil, nil
	}
	values := url.Values{
		"client_id": {p.clientID}, "format": {"json"}, "search": {request.Query},
		"limit": {strconv.Itoa(providerLimit(request.Limit))}, "include": {"musicinfo"},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v3.0/tracks/?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Results []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Duration        int64  `json:"duration"`
			ArtistName      string `json:"artist_name"`
			AlbumName       string `json:"album_name"`
			AlbumImage      string `json:"album_image"`
			ShareURL        string `json:"shareurl"`
			Audio           string `json:"audio"`
			AudioDownload   string `json:"audiodownload"`
			DownloadAllowed bool   `json:"audiodownload_allowed"`
			LicenseCCURL    string `json:"license_ccurl"`
			ReleaseDate     string `json:"releasedate"`
		} `json:"results"`
	}
	if err := getJSON(p.client, httpRequest, &response); err != nil {
		return nil, err
	}
	hits := make([]model.Hit, 0, len(response.Results))
	for _, item := range response.Results {
		score := textScore(request.Query, item.Name+" "+item.ArtistName+" "+item.AlbumName)
		offers := []model.Offer{{Type: "stream", Provider: "jamendo", Access: "free", URL: item.ShareURL, DirectMediaURL: item.Audio, Rights: model.OfferRights{Embeddable: true}}}
		if item.DownloadAllowed && item.AudioDownload != "" {
			offers = append(offers, model.Offer{Type: "download", Provider: "jamendo", Access: "free", URL: item.ShareURL, DirectMediaURL: item.AudioDownload, License: item.LicenseCCURL, Rights: model.OfferRights{Downloadable: true, AttributionRequired: true}})
		}
		hits = append(hits, model.Hit{
			EntityType:        "recording",
			Summary:           model.Summary{Title: item.Name, Artists: nonEmpty(item.ArtistName), PrimaryRelease: item.AlbumName, Date: item.ReleaseDate, DurationMS: item.Duration * 1000, ArtworkURL: item.AlbumImage},
			RankScore:         score,
			Match:             model.Match{Quality: textQuality(score), Fields: []string{"title", "artist", "release"}, Reasons: []string{"Jamendo independent-music search"}},
			Providers:         []string{"jamendo"},
			ProviderRefs:      []model.ProviderRef{{Provider: "jamendo", Type: "recording", ID: item.ID, URL: item.ShareURL}},
			AvailableSections: []string{"artwork", "offers"}, Offers: offers,
		})
	}
	return hits, nil
}

func (p *jamendo) Lookup(ctx context.Context, ref model.ProviderRef, include []string) (map[string]any, error) {
	if ref.Provider != "jamendo" || p.clientID == "" {
		return nil, ErrUnsupported
	}
	values := url.Values{"client_id": {p.clientID}, "format": {"json"}, "id": {ref.ID}, "include": {"musicinfo"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v3.0/tracks/?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := getJSON(p.client, request, &response); err != nil {
		return nil, err
	}
	return response, nil
}
