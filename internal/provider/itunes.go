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

type iTunes struct {
	client  *http.Client
	baseURL string
}

type iTunesResult struct {
	WrapperType       string  `json:"wrapperType"`
	Kind              string  `json:"kind"`
	ArtistID          int64   `json:"artistId"`
	CollectionID      int64   `json:"collectionId"`
	TrackID           int64   `json:"trackId"`
	ArtistName        string  `json:"artistName"`
	CollectionName    string  `json:"collectionName"`
	TrackName         string  `json:"trackName"`
	ArtistViewURL     string  `json:"artistViewUrl"`
	CollectionViewURL string  `json:"collectionViewUrl"`
	TrackViewURL      string  `json:"trackViewUrl"`
	PreviewURL        string  `json:"previewUrl"`
	ArtworkURL100     string  `json:"artworkUrl100"`
	ReleaseDate       string  `json:"releaseDate"`
	Country           string  `json:"country"`
	Currency          string  `json:"currency"`
	TrackTimeMillis   int64   `json:"trackTimeMillis"`
	TrackPrice        float64 `json:"trackPrice"`
	CollectionPrice   float64 `json:"collectionPrice"`
	PrimaryGenreName  string  `json:"primaryGenreName"`
}

func NewITunes(client *http.Client, cfg config.Config) Provider {
	return &iTunes{client: client, baseURL: cfg.ITunesBaseURL}
}

func (p *iTunes) Info() model.ProviderInfo {
	return model.ProviderInfo{ID: "itunes", Enabled: true, Configured: true, Capabilities: []string{"search:artist", "search:recording", "search:release", "lookup", "offers:preview", "offers:purchase"}}
}

func (p *iTunes) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if strings.TrimSpace(request.Query) == "" {
		return nil, nil
	}
	entities := make([]string, 0, 3)
	if wantsType(request.Types, "artist") {
		entities = append(entities, "musicArtist")
	}
	if wantsType(request.Types, "recording", "release_track") {
		entities = append(entities, "musicTrack")
	}
	if wantsType(request.Types, "release") {
		entities = append(entities, "album")
	}
	var hits []model.Hit
	var firstErr error
	for _, entity := range entities {
		items, err := p.search(ctx, request.Query, entity, request.Limit, request.Strategy.Region)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, item := range items {
			if hit, ok := p.toHit(item, request.Query, entity); ok {
				hits = append(hits, hit)
			}
		}
	}
	return hits, firstErr
}

func (p *iTunes) search(ctx context.Context, query, entity string, limit int, region string) ([]iTunesResult, error) {
	country := strings.ToUpper(strings.TrimSpace(region))
	if len(country) != 2 {
		country = "US"
	}
	values := url.Values{
		"term":    {query},
		"country": {country},
		"media":   {"music"},
		"entity":  {entity},
		"limit":   {strconv.Itoa(providerLimit(limit))},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/search?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Results []iTunesResult `json:"results"`
	}
	if err := getJSON(p.client, request, &response); err != nil {
		return nil, err
	}
	return response.Results, nil
}

func (p *iTunes) toHit(item iTunesResult, query, requestedEntity string) (model.Hit, bool) {
	var hit model.Hit
	matchText := ""
	switch requestedEntity {
	case "musicArtist":
		if item.ArtistID == 0 || item.ArtistName == "" {
			return hit, false
		}
		hit.EntityType = "artist"
		hit.Summary = model.Summary{Name: item.ArtistName}
		hit.ProviderRefs = []model.ProviderRef{{Provider: "itunes", Type: "artist", ID: strconv.FormatInt(item.ArtistID, 10), URL: item.ArtistViewURL}}
		hit.AvailableSections = []string{"recordings", "releases", "offers"}
		matchText = item.ArtistName
	case "album":
		if item.CollectionID == 0 || item.CollectionName == "" {
			return hit, false
		}
		hit.EntityType = "release"
		hit.Summary = model.Summary{Title: item.CollectionName, Artists: nonEmpty(item.ArtistName), Date: dateOnly(item.ReleaseDate), Country: item.Country, ArtworkURL: item.ArtworkURL100}
		hit.ProviderRefs = []model.ProviderRef{{Provider: "itunes", Type: "release", ID: strconv.FormatInt(item.CollectionID, 10), URL: item.CollectionViewURL}}
		hit.AvailableSections = []string{"tracks", "artwork", "offers"}
		matchText = item.CollectionName + " " + item.ArtistName
		if item.CollectionViewURL != "" {
			hit.Offers = append(hit.Offers, purchaseOffer("itunes", item.CollectionViewURL, item.CollectionPrice, item.Currency))
		}
	default:
		if item.TrackID == 0 || item.TrackName == "" {
			return hit, false
		}
		hit.EntityType = "recording"
		hit.Summary = model.Summary{Title: item.TrackName, Artists: nonEmpty(item.ArtistName), PrimaryRelease: item.CollectionName, Date: dateOnly(item.ReleaseDate), Country: item.Country, DurationMS: item.TrackTimeMillis, ArtworkURL: item.ArtworkURL100}
		hit.ProviderRefs = []model.ProviderRef{{Provider: "itunes", Type: "recording", ID: strconv.FormatInt(item.TrackID, 10), URL: item.TrackViewURL}}
		hit.AvailableSections = []string{"releases", "artwork", "offers"}
		matchText = item.TrackName + " " + item.ArtistName + " " + item.CollectionName
		if item.PreviewURL != "" {
			hit.Offers = append(hit.Offers, model.Offer{Type: "preview", Provider: "itunes", Access: "free", URL: item.TrackViewURL, DirectMediaURL: item.PreviewURL, Rights: model.OfferRights{Embeddable: true}})
		}
		if item.TrackViewURL != "" {
			hit.Offers = append(hit.Offers, purchaseOffer("itunes", item.TrackViewURL, item.TrackPrice, item.Currency))
		}
	}
	hit.RankScore = textScore(query, matchText)
	hit.Match = model.Match{Quality: textQuality(hit.RankScore), Fields: []string{"title", "artist", "release"}, Reasons: []string{"iTunes catalog search"}}
	hit.Providers = []string{"itunes"}
	return hit, true
}

func (p *iTunes) Lookup(ctx context.Context, ref model.ProviderRef, include []string) (map[string]any, error) {
	if ref.Provider != "itunes" {
		return nil, ErrUnsupported
	}
	values := url.Values{"id": {ref.ID}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/lookup?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := getJSON(p.client, request, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func purchaseOffer(provider, target string, price float64, currency string) model.Offer {
	offer := model.Offer{Type: "purchase", Provider: provider, Access: "paid", URL: target, Rights: model.OfferRights{}}
	if price > 0 && currency != "" {
		offer.Price = &model.Price{Amount: strconv.FormatFloat(price, 'f', 2, 64), Currency: currency}
	}
	return offer
}

func dateOnly(value string) string {
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func textScore(query, candidate string) float64 {
	query = normalizeText(query)
	candidate = normalizeText(candidate)
	if query == "" || candidate == "" {
		return 0.4
	}
	if query == candidate {
		return 1
	}
	if strings.Contains(candidate, query) {
		return 0.92
	}
	queryTerms := strings.Fields(query)
	candidateTerms := make(map[string]bool)
	for _, term := range strings.Fields(candidate) {
		candidateTerms[term] = true
	}
	matched := 0
	for _, term := range queryTerms {
		if candidateTerms[term] {
			matched++
		}
	}
	if len(queryTerms) == 0 {
		return 0.4
	}
	return 0.35 + 0.55*(float64(matched)/float64(len(queryTerms)))
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func textQuality(score float64) string {
	switch {
	case score >= 0.98:
		return "exact"
	case score >= 0.85:
		return "strong"
	case score >= 0.6:
		return "fuzzy"
	default:
		return "related"
	}
}
