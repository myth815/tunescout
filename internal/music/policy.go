package music

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/myth815/tunescout/internal/model"
)

type Policy struct{}

func (Policy) Domain() string { return "music" }

func (Policy) Prepare(request model.SearchRequest) (model.SearchRequest, model.InterpretedQuery, []string, error) {
	request.Query = strings.TrimSpace(request.Query)
	inputTypes := make([]string, 0, len(request.Inputs))
	parts := make([]string, 0, 8)
	if request.Query != "" {
		parts = append(parts, request.Query)
	}
	for _, input := range request.Inputs {
		inputType := strings.ToLower(strings.TrimSpace(input.Type))
		if inputType == "" {
			continue
		}
		inputTypes = append(inputTypes, inputType)
		if input.Text != "" {
			parts = append(parts, input.Text)
		}
		if input.Content != "" {
			parts = append(parts, excerpt(input.Content, 500))
		}
		if inputType == "metadata" {
			for _, key := range []string{"title", "artist", "album", "filename", "context"} {
				if value, ok := input.Fields[key]; ok {
					parts = append(parts, fmt.Sprint(value))
				}
			}
		}
	}
	if request.Query == "" {
		request.Query = strings.TrimSpace(strings.Join(parts, " "))
	}
	if request.Query == "" && request.AudioPath == "" {
		return request, model.InterpretedQuery{}, nil, errors.New("query or audio input is required")
	}
	if request.Limit <= 0 {
		request.Limit = 20
	}
	if request.Limit > 100 {
		request.Limit = 100
	}
	if len(request.Types) == 0 {
		request.Types = []string{"artist", "recording", "release"}
	}
	warnings := unknownFilterWarnings(request.Filters)
	intent := "catalog_search"
	if request.AudioPath != "" {
		intent = "identify_audio"
	} else {
		for _, inputType := range inputTypes {
			if inputType == "lyrics" {
				intent = "lyrics_search"
			}
		}
	}
	return request, model.InterpretedQuery{Original: request.Query, Normalized: normalize(request.Query), DetectedIntent: intent, InputTypes: unique(inputTypes)}, warnings, nil
}

func (Policy) MergeKey(hit model.Hit) string {
	name := hit.Summary.Title
	if name == "" {
		name = hit.Summary.Name
	}
	artist := ""
	if len(hit.Summary.Artists) > 0 {
		artist = hit.Summary.Artists[0]
	}
	return strings.Join([]string{hit.EntityType, normalize(name), normalize(artist)}, "|")
}

func (Policy) CanMerge(left, right model.Hit) bool {
	if left.EntityType != right.EntityType {
		return false
	}
	sharedProvider := false
	sharedIdentity := false
	for _, leftRef := range left.ProviderRefs {
		for _, rightRef := range right.ProviderRefs {
			if leftRef.Provider != rightRef.Provider {
				continue
			}
			sharedProvider = true
			if leftRef.Type == rightRef.Type && leftRef.ID == rightRef.ID {
				sharedIdentity = true
			}
		}
	}
	if sharedProvider {
		return sharedIdentity
	}
	return compatibleArtists(left.Summary.Artists, right.Summary.Artists)
}

func (Policy) Merge(left, right model.Hit) model.Hit {
	if right.RankScore > left.RankScore {
		left.Summary = richerSummary(right.Summary, left.Summary)
		left.Match = right.Match
		left.RankScore = right.RankScore
	} else {
		left.Summary = richerSummary(left.Summary, right.Summary)
	}
	left.Providers = unique(append(left.Providers, right.Providers...))
	left.ProviderRefs = uniqueRefs(append(left.ProviderRefs, right.ProviderRefs...))
	left.AvailableSections = unique(append(left.AvailableSections, right.AvailableSections...))
	left.Offers = append(left.Offers, right.Offers...)
	left.Evidence = append(left.Evidence, right.Evidence...)
	left.ExternalIDs = mergeMaps(left.ExternalIDs, right.ExternalIDs)
	if right.IdentityConfidence != nil && (left.IdentityConfidence == nil || right.IdentityConfidence.Value > left.IdentityConfidence.Value) {
		left.IdentityConfidence = right.IdentityConfidence
	}
	providerBonus := 0.02 * float64(len(left.Providers)-1)
	if providerBonus > 0.08 {
		providerBonus = 0.08
	}
	left.RankScore = min(1, left.RankScore+providerBonus)
	left.Match.Reasons = unique(append(left.Match.Reasons, "multiple providers converged on this entity"))
	return left
}

func (Policy) Finalize(hits []model.Hit, request model.SearchRequest) []model.Hit {
	hits = applyFilters(hits, request.Filters)
	for index := range hits {
		hits[index].RankScore = round(hits[index].RankScore)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].RankScore == hits[j].RankScore {
			return hits[i].EntityType < hits[j].EntityType
		}
		return hits[i].RankScore > hits[j].RankScore
	})
	if request.Limit > 0 && len(hits) > request.Limit {
		return hits[:request.Limit]
	}
	return hits
}

func compatibleArtists(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return true
	}
	for _, a := range left {
		for _, b := range right {
			if normalize(a) == normalize(b) {
				return true
			}
		}
	}
	return false
}

func applyFilters(hits []model.Hit, filters map[string]any) []model.Hit {
	if len(filters) == 0 {
		return hits
	}
	result := make([]model.Hit, 0, len(hits))
	for _, hit := range hits {
		if !containsFilter(hit.Summary.Title+" "+hit.Summary.Name, filters["title"]) ||
			!containsFilter(strings.Join(hit.Summary.Artists, " "), filters["artist"]) ||
			!containsFilter(hit.Summary.PrimaryRelease, firstFilter(filters, "release", "album")) ||
			!durationFilter(hit.Summary.DurationMS, filters) {
			continue
		}
		result = append(result, hit)
	}
	return result
}

func containsFilter(value string, filter any) bool {
	if filter == nil || strings.TrimSpace(fmt.Sprint(filter)) == "" {
		return true
	}
	return strings.Contains(normalize(value), normalize(fmt.Sprint(filter)))
}

func firstFilter(filters map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := filters[key]; ok {
			return value
		}
	}
	return nil
}

func durationFilter(duration int64, filters map[string]any) bool {
	minimum, hasMinimum := number(filters["duration_min_ms"])
	maximum, hasMaximum := number(filters["duration_max_ms"])
	if duration == 0 && (hasMinimum || hasMaximum) {
		return false
	}
	return (!hasMinimum || float64(duration) >= minimum) && (!hasMaximum || float64(duration) <= maximum)
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func unknownFilterWarnings(filters map[string]any) []string {
	known := map[string]bool{"title": true, "artist": true, "release": true, "album": true, "duration_min_ms": true, "duration_max_ms": true}
	var warnings []string
	for key := range filters {
		if !known[key] {
			warnings = append(warnings, "unsupported filter ignored: "+key)
		}
	}
	sort.Strings(warnings)
	return warnings
}

func richerSummary(primary, fallback model.Summary) model.Summary {
	if primary.Name == "" {
		primary.Name = fallback.Name
	}
	if primary.Title == "" {
		primary.Title = fallback.Title
	}
	if primary.SortName == "" {
		primary.SortName = fallback.SortName
	}
	if len(primary.Artists) == 0 {
		primary.Artists = fallback.Artists
	}
	if len(primary.Aliases) == 0 {
		primary.Aliases = fallback.Aliases
	}
	if primary.PrimaryRelease == "" {
		primary.PrimaryRelease = fallback.PrimaryRelease
	}
	if primary.Date == "" {
		primary.Date = fallback.Date
	}
	if primary.Country == "" {
		primary.Country = fallback.Country
	}
	if primary.DurationMS == 0 {
		primary.DurationMS = fallback.DurationMS
	}
	if primary.ArtworkURL == "" {
		primary.ArtworkURL = fallback.ArtworkURL
	}
	if primary.Disambiguation == "" {
		primary.Disambiguation = fallback.Disambiguation
	}
	return primary
}

func normalize(value string) string {
	var builder strings.Builder
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			space = false
		} else if !space {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func excerpt(value string, maximum int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maximum {
		return string(runes)
	}
	return string(runes[:maximum])
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func uniqueRefs(values []model.ProviderRef) []model.ProviderRef {
	seen := make(map[string]bool, len(values))
	result := make([]model.ProviderRef, 0, len(values))
	for _, value := range values {
		key := value.Provider + "|" + value.Type + "|" + value.ID
		if value.ID != "" && !seen[key] {
			seen[key] = true
			result = append(result, value)
		}
	}
	return result
}

func mergeMaps(left, right map[string]any) map[string]any {
	if left == nil && right == nil {
		return nil
	}
	result := make(map[string]any, len(left)+len(right))
	for key, value := range right {
		result[key] = value
	}
	for key, value := range left {
		result[key] = value
	}
	return result
}

func round(value float64) float64 { return float64(int(value*1000+0.5)) / 1000 }
