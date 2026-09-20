// Package federation contains domain-neutral orchestration for querying multiple
// upstream providers. Domain packages supply query interpretation and entity
// merge rules; a future film service can reuse this package without importing
// TuneScout's music-specific adapters.
package federation

import "github.com/myth815/tunescout/internal/model"

type DomainPolicy interface {
	Domain() string
	Prepare(model.SearchRequest) (model.SearchRequest, model.InterpretedQuery, []string, error)
	MergeKey(model.Hit) string
	CanMerge(model.Hit, model.Hit) bool
	Merge(model.Hit, model.Hit) model.Hit
	Finalize([]model.Hit, model.SearchRequest) []model.Hit
}
