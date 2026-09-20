package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

type acoustID struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func NewAcoustID(client *http.Client, cfg config.Config) Provider {
	return &acoustID{client: client, baseURL: cfg.AcoustIDBaseURL, apiKey: cfg.AcoustIDAPIKey}
}

func (p *acoustID) Info() model.ProviderInfo {
	configured := p.apiKey != ""
	return model.ProviderInfo{ID: "acoustid", Enabled: configured, Configured: configured, RequiresKey: true, Capabilities: []string{"identify:audio"}}
}

func (p *acoustID) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if p.apiKey == "" || request.AudioPath == "" || !wantsType(request.Types, "recording", "release_track") {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "fpcalc", "-json", request.AudioPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("generate chromaprint fingerprint: %w", err)
	}
	var fingerprint struct {
		Duration    float64 `json:"duration"`
		Fingerprint string  `json:"fingerprint"`
	}
	if err := json.Unmarshal(output, &fingerprint); err != nil {
		return nil, fmt.Errorf("decode fpcalc output: %w", err)
	}
	values := url.Values{
		"client":      {p.apiKey},
		"meta":        {"recordings+releasegroups+compress"},
		"duration":    {strconv.Itoa(int(fingerprint.Duration + 0.5))},
		"fingerprint": {fingerprint.Fingerprint},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v2/lookup?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Status  string `json:"status"`
		Results []struct {
			ID         string  `json:"id"`
			Score      float64 `json:"score"`
			Recordings []struct {
				ID       string  `json:"id"`
				Title    string  `json:"title"`
				Duration float64 `json:"duration"`
				Artists  []struct {
					Name string `json:"name"`
				} `json:"artists"`
				ReleaseGroups []struct {
					Title string `json:"title"`
				} `json:"releasegroups"`
			} `json:"recordings"`
		} `json:"results"`
	}
	if err := getJSON(p.client, httpRequest, &response); err != nil {
		return nil, err
	}
	if response.Status != "ok" {
		return nil, fmt.Errorf("acoustid returned status %q", response.Status)
	}
	var hits []model.Hit
	for _, result := range response.Results {
		for _, recording := range result.Recordings {
			artists := make([]string, 0, len(recording.Artists))
			for _, artist := range recording.Artists {
				artists = append(artists, artist.Name)
			}
			release := ""
			if len(recording.ReleaseGroups) > 0 {
				release = recording.ReleaseGroups[0].Title
			}
			level := "medium"
			if result.Score >= 0.9 {
				level = "high"
			}
			hits = append(hits, model.Hit{
				EntityType:         "recording",
				Summary:            model.Summary{Title: recording.Title, Artists: artists, PrimaryRelease: release, DurationMS: int64(recording.Duration * 1000)},
				RankScore:          result.Score,
				IdentityConfidence: &model.Confidence{Value: result.Score, Level: level, Basis: []string{"audio_fingerprint"}},
				Match:              model.Match{Quality: "fingerprint", Fields: []string{"audio"}, Reasons: []string{"Chromaprint fingerprint matched AcoustID"}},
				Providers:          []string{"acoustid"},
				ProviderRefs:       []model.ProviderRef{{Provider: "musicbrainz", Type: "recording", ID: recording.ID, URL: "https://musicbrainz.org/recording/" + recording.ID}},
				ExternalIDs:        map[string]any{"musicbrainz_recording_id": recording.ID, "acoustid": result.ID},
				AvailableSections:  []string{"releases", "credits", "external_ids"},
				Evidence:           []model.Evidence{{Type: "audio_fingerprint", Provider: "acoustid", Score: result.Score, ProviderID: result.ID}},
			})
		}
	}
	return hits, nil
}

func (p *acoustID) Lookup(context.Context, model.ProviderRef, []string) (map[string]any, error) {
	return nil, ErrUnsupported
}
