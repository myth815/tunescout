package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/model"
)

func TestAudiusArtworkIgnoresMirrorList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"track-1","title":"Song","duration":123,"permalink":"/artist/song","artwork":{"1000x1000":"https://example.test/cover.jpg","mirrors":["https://mirror.test"]},"user":{"name":"Artist"}}]}`))
	}))
	defer server.Close()

	item := NewAudius(server.Client(), config.Config{AudiusBaseURL: server.URL})
	hits, err := item.Search(context.Background(), model.SearchRequest{Query: "Song Artist", Types: []string{"recording"}, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1", len(hits))
	}
	if hits[0].Summary.ArtworkURL != "https://example.test/cover.jpg" {
		t.Fatalf("artwork URL = %q", hits[0].Summary.ArtworkURL)
	}
}
