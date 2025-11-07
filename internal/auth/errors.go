package auth

// Error codes for authentication
const (
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeInvalidAPIKey = "INVALID_API_KEY"
	ErrCodeMissingAPIKey = "MISSING_API_KEY"
)

// Error messages
const (
	ErrMsgMissingAPIKey = "Missing X-API-Key header"
	ErrMsgInvalidAPIKey = "Invalid or expired API key"
	ErrMsgUnauthorized  = "Authentication required"
)
