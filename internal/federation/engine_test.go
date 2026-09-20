package federation_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/myth815/tunescout/internal/federation"
	"github.com/myth815/tunescout/internal/model"
	"github.com/myth815/tunescout/internal/music"
	"github.com/myth815/tunescout/internal/provider"
)

type fakeProvider struct {
	id   string
	hits []model.Hit
	err  error
}

func (p fakeProvider) Info() model.ProviderInfo {
	return model.ProviderInfo{ID: p.id, Enabled: true, Configured: true, Capabilities: []string{"search:recording"}}
}

func (p fakeProvider) Search(context.Context, model.SearchRequest) ([]model.Hit, error) {
	return p.hits, p.err
}

func (p fakeProvider) Lookup(context.Context, model.ProviderRef, []string) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func TestSearchKeepsHitsAcrossProviderFailures(t *testing.T) {
	hit := model.Hit{
		EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}}, RankScore: 0.9,
		Providers: []string{"working"}, ProviderRefs: []model.ProviderRef{{Provider: "working", Type: "recording", ID: "one"}},
	}
	providers := []provider.Provider{
		fakeProvider{id: "working", hits: []model.Hit{hit}},
		fakeProvider{id: "broken", err: errors.New("upstream unavailable")},
	}
	engine := federation.New(
		providers,
		music.Policy{}, 2*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	response, err := engine.Search(context.Background(), model.SearchRequest{Query: "红豆", Types: []string{"recording"}})
	if err != nil {
		t.Fatal(err)
	}
	if !response.Partial || len(response.Hits) != 1 || response.Hits[0].EntityRef == "" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestSearchKeepsPartialProviderHits(t *testing.T) {
	hit := model.Hit{
		EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}}, RankScore: 0.9,
		Providers: []string{"mixed"}, ProviderRefs: []model.ProviderRef{{Provider: "mixed", Type: "recording", ID: "one"}},
	}
	engine := federation.New(
		[]provider.Provider{fakeProvider{id: "mixed", hits: []model.Hit{hit}, err: errors.New("one resource type failed")}},
		music.Policy{}, 2*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	response, err := engine.Search(context.Background(), model.SearchRequest{Query: "红豆", Types: []string{"recording"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Hits) != 1 || len(response.ProviderStatus) != 1 || response.ProviderStatus[0].Status != "partial" {
		t.Fatalf("unexpected partial response: %#v", response)
	}
}
