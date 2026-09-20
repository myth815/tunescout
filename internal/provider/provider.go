package provider

import (
	"context"
	"errors"

	"github.com/myth815/tunescout/internal/model"
)

var ErrUnsupported = errors.New("provider does not support this operation")

type Provider interface {
	Info() model.ProviderInfo
	Search(context.Context, model.SearchRequest) ([]model.Hit, error)
	Lookup(context.Context, model.ProviderRef, []string) (map[string]any, error)
}

func wantsType(types []string, values ...string) bool {
	if len(types) == 0 {
		return true
	}
	for _, requested := range types {
		for _, value := range values {
			if requested == value {
				return true
			}
		}
	}
	return false
}
