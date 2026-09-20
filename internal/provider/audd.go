package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

type audD struct {
	client   *http.Client
	baseURL  string
	apiToken string
}

func NewAudD(client *http.Client, cfg config.Config) Provider {
	return &audD{client: client, baseURL: cfg.AudDBaseURL, apiToken: cfg.AudDAPIToken}
}

func (p *audD) Info() model.ProviderInfo {
	configured := p.apiToken != ""
	return model.ProviderInfo{ID: "audd", Enabled: configured, Configured: configured, RequiresKey: true, Capabilities: []string{"identify:audio", "offers:source"}}
}

func (p *audD) Search(ctx context.Context, request model.SearchRequest) ([]model.Hit, error) {
	if p.apiToken == "" || request.AudioPath == "" || !wantsType(request.Types, "recording", "release_track") {
		return nil, nil
	}
	file, err := os.Open(request.AudioPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("api_token", p.apiToken); err != nil {
		return nil, err
	}
	if err := writer.WriteField("return", "apple_music,spotify,musicbrainz"); err != nil {
		return nil, err
	}
	part, err := writer.CreateFormFile("file", filepath.Base(request.AudioPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/", &body)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := p.client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned %s", response.Status)
	}
	var payload struct {
		Status string         `json:"status"`
		Error  map[string]any `json:"error"`
		Result *struct {
			Artist      string `json:"artist"`
			Title       string `json:"title"`
			Album       string `json:"album"`
			ReleaseDate string `json:"release_date"`
			Label       string `json:"label"`
			SongLink    string `json:"song_link"`
			Timecode    string `json:"timecode"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("audd returned status %q", payload.Status)
	}
	if payload.Result == nil {
		return nil, nil
	}
	result := payload.Result
	hit := model.Hit{
		EntityType:         "recording",
		Summary:            model.Summary{Title: result.Title, Artists: nonEmpty(result.Artist), PrimaryRelease: result.Album, Date: result.ReleaseDate},
		RankScore:          0.96,
		IdentityConfidence: &model.Confidence{Value: 0.96, Level: "high", Basis: []string{"commercial_audio_recognition"}},
		Match:              model.Match{Quality: "audio_recognition", Fields: []string{"audio"}, Reasons: []string{"AudD recognized the uploaded audio"}},
		Providers:          []string{"audd"},
		ProviderRefs:       []model.ProviderRef{{Provider: "audd", Type: "recording", ID: strings.TrimSpace(result.Artist + "::" + result.Title), URL: result.SongLink}},
		AvailableSections:  []string{"offers"},
		Evidence:           []model.Evidence{{Type: "audio_recognition", Provider: "audd", Value: result.Artist + " — " + result.Title, URL: result.SongLink}},
	}
	if result.SongLink != "" {
		hit.Offers = []model.Offer{{Type: "source_page", Provider: "audd", Access: "unknown", URL: result.SongLink, Rights: model.OfferRights{}}}
	}
	return []model.Hit{hit}, nil
}

func (p *audD) Lookup(context.Context, model.ProviderRef, []string) (map[string]any, error) {
	return nil, ErrUnsupported
}
