package federation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/myth815/tunescout/internal/entityref"
	"github.com/myth815/tunescout/internal/model"
	"github.com/myth815/tunescout/internal/provider"
)

type Engine struct {
	providers []provider.Provider
	byID      map[string]provider.Provider
	policy    DomainPolicy
	timeout   time.Duration
	logger    *slog.Logger
}

func New(providers []provider.Provider, policy DomainPolicy, timeout time.Duration, logger *slog.Logger) *Engine {
	byID := make(map[string]provider.Provider, len(providers))
	for _, item := range providers {
		byID[item.Info().ID] = item
	}
	return &Engine{providers: providers, byID: byID, policy: policy, timeout: timeout, logger: logger}
}

func (e *Engine) Search(ctx context.Context, request model.SearchRequest) (model.SearchResponse, error) {
	started := time.Now()
	prepared, interpreted, warnings, err := e.policy.Prepare(request)
	if err != nil {
		return model.SearchResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	type result struct {
		provider string
		hits     []model.Hit
		err      error
		elapsed  time.Duration
	}
	results := make(chan result, len(e.providers))
	selected := 0
	audioProviders := make(map[string]bool)
	for _, item := range e.providers {
		info := item.Info()
		if !info.Enabled || !providerSelected(prepared.Strategy.Providers, info.ID) {
			continue
		}
		for _, capability := range info.Capabilities {
			if capability == "identify:audio" {
				audioProviders[info.ID] = true
			}
		}
		selected++
		go func(current provider.Provider, id string) {
			providerStarted := time.Now()
			hits, callErr := current.Search(ctx, prepared)
			results <- result{provider: id, hits: hits, err: callErr, elapsed: time.Since(providerStarted)}
		}(item, info.ID)
	}

	buckets := make(map[string][]model.Hit)
	statuses := make([]model.ProviderRunStatus, 0, selected)
	partial := false
	audioAnalyzed := false
	if selected == 0 {
		warnings = append(warnings, "no enabled provider matched the requested provider selection")
	}
	if prepared.AudioPath != "" && len(audioProviders) == 0 {
		warnings = append(warnings, "audio was supplied but no audio-identification provider is enabled")
	}
	for range selected {
		current := <-results
		status := model.ProviderRunStatus{Provider: current.provider, Status: "success", ResultCount: len(current.hits), ElapsedMS: current.elapsed.Milliseconds()}
		if current.err != nil {
			status.Status = "error"
			if len(current.hits) > 0 {
				status.Status = "partial"
			}
			status.Error = current.err.Error()
			partial = true
			e.logger.Warn("provider search failed", "provider", current.provider, "error", current.err)
		}
		if prepared.AudioPath != "" && audioProviders[current.provider] && current.err == nil {
			audioAnalyzed = true
		}
		for _, hit := range current.hits {
			key := e.policy.MergeKey(hit)
			if key == "" {
				key = current.provider + ":" + firstRefID(hit.ProviderRefs)
			}
			clustered := false
			for index, existing := range buckets[key] {
				if e.policy.CanMerge(existing, hit) {
					buckets[key][index] = e.policy.Merge(existing, hit)
					clustered = true
					break
				}
			}
			if !clustered {
				buckets[key] = append(buckets[key], hit)
			}
		}
		statuses = append(statuses, status)
	}

	hits := make([]model.Hit, 0, len(buckets))
	for _, bucket := range buckets {
		for _, hit := range bucket {
			ref, refErr := entityref.Encode(e.policy.Domain(), hit.EntityType, hit.ProviderRefs)
			if refErr != nil {
				return model.SearchResponse{}, refErr
			}
			hit.EntityRef = ref
			hits = append(hits, hit)
		}
	}
	hits = e.policy.Finalize(hits, prepared)
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Provider < statuses[j].Provider })

	return model.SearchResponse{
		SearchID: "search_" + randomID(), Status: "completed", Partial: partial,
		Interpreted: interpreted, Hits: hits, ProviderStatus: statuses, Warnings: warnings,
		ElapsedMS: time.Since(started).Milliseconds(), AudioAnalyzed: audioAnalyzed, ExecutedAt: time.Now().UTC(),
	}, nil
}

func (e *Engine) Lookup(ctx context.Context, entityRef string, include []string) (model.EntityResponse, error) {
	locator, err := entityref.Decode(entityRef)
	if err != nil {
		return model.EntityResponse{}, err
	}
	if locator.Domain != e.policy.Domain() {
		return model.EntityResponse{}, errors.New("entity reference belongs to another domain")
	}
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	type result struct {
		index int
		value model.EntitySourceResult
	}
	results := make(chan result, len(locator.ProviderRefs))
	for index, ref := range locator.ProviderRefs {
		go func(i int, current model.ProviderRef) {
			item := e.byID[current.Provider]
			if item == nil {
				results <- result{index: i, value: model.EntitySourceResult{Provider: current.Provider, Status: "unsupported", Error: "provider is not available"}}
				return
			}
			data, lookupErr := item.Lookup(ctx, current, include)
			value := model.EntitySourceResult{Provider: current.Provider, Status: "success", Data: data}
			if lookupErr != nil {
				value.Status = "error"
				value.Error = lookupErr.Error()
			}
			results <- result{index: i, value: value}
		}(index, ref)
	}
	sources := make([]model.EntitySourceResult, len(locator.ProviderRefs))
	partial := false
	for range locator.ProviderRefs {
		current := <-results
		sources[current.index] = current.value
		if current.value.Status != "success" {
			partial = true
		}
	}
	return model.EntityResponse{EntityRef: entityRef, EntityType: locator.EntityType, Sources: sources, Partial: partial}, nil
}

func (e *Engine) Providers() []model.ProviderInfo {
	result := make([]model.ProviderInfo, 0, len(e.providers))
	for _, item := range e.providers {
		result = append(result, item.Info())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func providerSelected(selected []string, id string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, value := range selected {
		if value == "*" || strings.EqualFold(value, id) {
			return true
		}
	}
	return false
}

func firstRefID(refs []model.ProviderRef) string {
	if len(refs) == 0 {
		return randomID()
	}
	return refs[0].ID
}

func randomID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buffer)
}
