package model

import "time"

type SearchRequest struct {
	Query    string         `json:"query,omitempty"`
	Inputs   []Input        `json:"inputs,omitempty"`
	Mode     string         `json:"mode,omitempty"`
	Types    []string       `json:"types,omitempty"`
	Filters  map[string]any `json:"filters,omitempty"`
	Limit    int            `json:"limit,omitempty"`
	Locale   string         `json:"locale,omitempty"`
	Strategy Strategy       `json:"strategy,omitempty"`

	AudioPath string `json:"-"`
}

type Input struct {
	Type     string         `json:"type"`
	Role     string         `json:"role,omitempty"`
	Text     string         `json:"text,omitempty"`
	Content  string         `json:"content,omitempty"`
	Language string         `json:"language,omitempty"`
	Fields   map[string]any `json:"fields,omitempty"`
}

type Strategy struct {
	Depth     string   `json:"depth,omitempty"`
	Providers []string `json:"providers,omitempty"`
	Region    string   `json:"region,omitempty"`
}

type SearchResponse struct {
	SearchID       string              `json:"search_id"`
	Status         string              `json:"status"`
	Partial        bool                `json:"partial"`
	Interpreted    InterpretedQuery    `json:"interpreted_query"`
	Hits           []Hit               `json:"hits"`
	ProviderStatus []ProviderRunStatus `json:"provider_status"`
	Warnings       []string            `json:"warnings,omitempty"`
	ElapsedMS      int64               `json:"elapsed_ms"`
	NextCursor     string              `json:"next_cursor,omitempty"`
	AudioAnalyzed  bool                `json:"audio_analyzed,omitempty"`
	ExecutedAt     time.Time           `json:"executed_at"`
}

type InterpretedQuery struct {
	Original       string   `json:"original,omitempty"`
	Normalized     string   `json:"normalized,omitempty"`
	DetectedIntent string   `json:"detected_intent"`
	InputTypes     []string `json:"input_types,omitempty"`
}

type Hit struct {
	EntityRef          string         `json:"entity_ref,omitempty"`
	EntityType         string         `json:"entity_type"`
	Summary            Summary        `json:"summary"`
	RankScore          float64        `json:"rank_score"`
	IdentityConfidence *Confidence    `json:"identity_confidence,omitempty"`
	Match              Match          `json:"match"`
	Providers          []string       `json:"providers"`
	ProviderRefs       []ProviderRef  `json:"provider_refs"`
	ExternalIDs        map[string]any `json:"external_ids,omitempty"`
	AvailableSections  []string       `json:"available_sections,omitempty"`
	Offers             []Offer        `json:"offers,omitempty"`
	Evidence           []Evidence     `json:"evidence,omitempty"`
	Conflicts          []Conflict     `json:"conflicts,omitempty"`
}

type Summary struct {
	Name           string   `json:"name,omitempty"`
	Title          string   `json:"title,omitempty"`
	SortName       string   `json:"sort_name,omitempty"`
	Artists        []string `json:"artists,omitempty"`
	Aliases        []string `json:"aliases,omitempty"`
	PrimaryRelease string   `json:"primary_release,omitempty"`
	Date           string   `json:"date,omitempty"`
	Country        string   `json:"country,omitempty"`
	DurationMS     int64    `json:"duration_ms,omitempty"`
	ArtworkURL     string   `json:"artwork_url,omitempty"`
	Disambiguation string   `json:"disambiguation,omitempty"`
}

type Confidence struct {
	Value float64  `json:"value"`
	Level string   `json:"level"`
	Basis []string `json:"basis"`
}

type Match struct {
	Quality string   `json:"quality"`
	Fields  []string `json:"fields,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type ProviderRef struct {
	Provider string `json:"provider"`
	Type     string `json:"type"`
	ID       string `json:"id"`
	URL      string `json:"url,omitempty"`
}

type Offer struct {
	Type           string      `json:"type"`
	Provider       string      `json:"provider"`
	Access         string      `json:"access"`
	URL            string      `json:"url,omitempty"`
	DirectMediaURL string      `json:"direct_media_url,omitempty"`
	DurationMS     int64       `json:"duration_ms,omitempty"`
	Regions        []string    `json:"regions,omitempty"`
	ExpiresAt      *time.Time  `json:"expires_at,omitempty"`
	License        string      `json:"license,omitempty"`
	Price          *Price      `json:"price,omitempty"`
	Rights         OfferRights `json:"rights"`
}

type Price struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type OfferRights struct {
	Embeddable          bool `json:"embeddable"`
	Downloadable        bool `json:"downloadable"`
	AttributionRequired bool `json:"attribution_required"`
}

type Evidence struct {
	Type       string  `json:"type"`
	Provider   string  `json:"provider,omitempty"`
	Value      string  `json:"value,omitempty"`
	Score      float64 `json:"score,omitempty"`
	ProviderID string  `json:"provider_id,omitempty"`
	URL        string  `json:"url,omitempty"`
}

type Conflict struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

type ProviderRunStatus struct {
	Provider    string `json:"provider"`
	Status      string `json:"status"`
	ResultCount int    `json:"result_count,omitempty"`
	ElapsedMS   int64  `json:"elapsed_ms"`
	Error       string `json:"error,omitempty"`
}

type ProviderInfo struct {
	ID           string   `json:"id"`
	Enabled      bool     `json:"enabled"`
	Configured   bool     `json:"configured"`
	Capabilities []string `json:"capabilities"`
	RequiresKey  bool     `json:"requires_key"`
}

type EntityLocator struct {
	Version      int           `json:"version"`
	Domain       string        `json:"domain"`
	EntityType   string        `json:"entity_type"`
	ProviderRefs []ProviderRef `json:"provider_refs"`
}

type EntityResponse struct {
	EntityRef  string               `json:"entity_ref"`
	EntityType string               `json:"entity_type"`
	Sources    []EntitySourceResult `json:"sources"`
	Partial    bool                 `json:"partial"`
}

type EntitySourceResult struct {
	Provider string         `json:"provider"`
	Status   string         `json:"status"`
	Data     map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
}
