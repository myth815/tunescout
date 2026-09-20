package music

import (
	"testing"

	"github.com/myth815/tunescout/internal/model"
)

func TestMergeConvergedProviders(t *testing.T) {
	policy := Policy{}
	left := model.Hit{EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}}, RankScore: 0.8, Providers: []string{"musicbrainz"}, ProviderRefs: []model.ProviderRef{{Provider: "musicbrainz", Type: "recording", ID: "1"}}}
	right := model.Hit{EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}, ArtworkURL: "cover"}, RankScore: 0.9, Providers: []string{"itunes"}, ProviderRefs: []model.ProviderRef{{Provider: "itunes", Type: "recording", ID: "2"}}}
	got := policy.Merge(left, right)
	if len(got.Providers) != 2 || len(got.ProviderRefs) != 2 || got.Summary.ArtworkURL != "cover" {
		t.Fatalf("unexpected merge: %#v", got)
	}
}

func TestCanMergeRejectsDifferentIDsFromSameProvider(t *testing.T) {
	policy := Policy{}
	left := model.Hit{EntityType: "artist", Summary: model.Summary{Name: "王菲"}, ProviderRefs: []model.ProviderRef{{Provider: "musicbrainz", Type: "artist", ID: "one"}}}
	right := model.Hit{EntityType: "artist", Summary: model.Summary{Name: "王菲"}, ProviderRefs: []model.ProviderRef{{Provider: "musicbrainz", Type: "artist", ID: "two"}}}
	if policy.CanMerge(left, right) {
		t.Fatal("different identities from the same provider must not be merged")
	}
}

func TestCanMergeAcceptsCorroboratingProviders(t *testing.T) {
	policy := Policy{}
	left := model.Hit{EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}}, ProviderRefs: []model.ProviderRef{{Provider: "musicbrainz", Type: "recording", ID: "one"}}}
	right := model.Hit{EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}}, ProviderRefs: []model.ProviderRef{{Provider: "itunes", Type: "recording", ID: "two"}}}
	if !policy.CanMerge(left, right) {
		t.Fatal("matching identities from independent providers should merge")
	}
}

func TestFinalizeAppliesMusicFilters(t *testing.T) {
	policy := Policy{}
	hits := []model.Hit{
		{EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"王菲"}, DurationMS: 250000}, RankScore: 0.9},
		{EntityType: "recording", Summary: model.Summary{Title: "红豆", Artists: []string{"方大同"}, DurationMS: 240000}, RankScore: 0.8},
	}
	got := policy.Finalize(hits, model.SearchRequest{Limit: 10, Filters: map[string]any{"artist": "王菲", "duration_min_ms": float64(245000)}})
	if len(got) != 1 || got[0].Summary.Artists[0] != "王菲" {
		t.Fatalf("unexpected filtered hits: %#v", got)
	}
}

func TestPrepareDeepSearchKeepsCallerLimit(t *testing.T) {
	prepared, _, _, err := (Policy{}).Prepare(model.SearchRequest{Query: "王菲", Limit: 5, Strategy: model.Strategy{Depth: "deep"}})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Limit != 25 || prepared.ResultLimit != 5 {
		t.Fatalf("unexpected deep limits: provider=%d result=%d", prepared.Limit, prepared.ResultLimit)
	}
}

func TestPrepareRejectsUnknownEntityType(t *testing.T) {
	_, _, _, err := (Policy{}).Prepare(model.SearchRequest{Query: "王菲", Types: []string{"film"}})
	if err == nil {
		t.Fatal("unknown entity type should fail")
	}
}
