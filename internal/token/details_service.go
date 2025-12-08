package token

import "context"

// DetailsService defines the interface for retrieving token details.
type DetailsService interface {
	// GetTokenDetails retrieves the token details for the given contract address.
	// If no details are found, it returns nil without an error.
	GetTokenDetails(ctx context.Context, contractAddress string) (*Details, error)
}
