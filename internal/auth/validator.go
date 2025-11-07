package auth

import "strings"

// APIKeyValidator validates API keys for authentication
type APIKeyValidator struct {
	validKeys map[string]bool
	enabled   bool
}

// NewAPIKeyValidator creates a new API key validator
func NewAPIKeyValidator(keys []string, enabled bool) *APIKeyValidator {
	validKeysMap := make(map[string]bool)
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			validKeysMap[trimmed] = true
		}
	}

	return &APIKeyValidator{
		validKeys: validKeysMap,
		enabled:   enabled,
	}
}

// ValidateKey checks if the provided API key is valid
func (v *APIKeyValidator) ValidateKey(key string) bool {
	if !v.enabled {
		return true // Auth disabled, all keys valid
	}

	if key == "" {
		return false
	}

	return v.validKeys[key]
}

// IsEnabled returns whether authentication is enabled
func (v *APIKeyValidator) IsEnabled() bool {
	return v.enabled
}
