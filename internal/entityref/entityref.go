package entityref

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/myth815/tunescout/internal/model"
)

const prefix = "ts1_"

func Encode(domain, entityType string, refs []model.ProviderRef) (string, error) {
	if strings.TrimSpace(domain) == "" || strings.TrimSpace(entityType) == "" || len(refs) == 0 || len(refs) > 12 {
		return "", errors.New("invalid entity locator")
	}
	payload, err := json.Marshal(model.EntityLocator{Version: 1, Domain: domain, EntityType: entityType, ProviderRefs: refs})
	if err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func Decode(value string) (model.EntityLocator, error) {
	if len(value) > 8192 || !strings.HasPrefix(value, prefix) {
		return model.EntityLocator{}, errors.New("unsupported entity reference")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return model.EntityLocator{}, errors.New("invalid entity reference encoding")
	}
	var locator model.EntityLocator
	if err := json.Unmarshal(payload, &locator); err != nil {
		return model.EntityLocator{}, errors.New("invalid entity reference payload")
	}
	if locator.Version != 1 || locator.Domain == "" || locator.EntityType == "" || len(locator.ProviderRefs) == 0 || len(locator.ProviderRefs) > 12 {
		return model.EntityLocator{}, errors.New("incomplete entity reference")
	}
	for _, ref := range locator.ProviderRefs {
		if ref.Provider == "" || ref.Type == "" || ref.ID == "" || len(ref.Provider) > 64 || len(ref.Type) > 64 || len(ref.ID) > 512 {
			return model.EntityLocator{}, errors.New("invalid provider reference")
		}
	}
	return locator, nil
}
