package entityref

import (
	"testing"

	"github.com/myth815/tunescout/internal/model"
)

func TestRoundTrip(t *testing.T) {
	want := []model.ProviderRef{{Provider: "musicbrainz", Type: "recording", ID: "abc"}}
	encoded, err := Encode("music", "recording", want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.EntityType != "recording" || len(got.ProviderRefs) != 1 || got.ProviderRefs[0].ID != "abc" {
		t.Fatalf("unexpected locator: %#v", got)
	}
}

func TestRejectsOversizedReference(t *testing.T) {
	if _, err := Decode("ts1_" + string(make([]byte, 9000))); err == nil {
		t.Fatal("oversized entity reference should be rejected")
	}
}
