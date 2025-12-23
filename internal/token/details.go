package token

// Details describes the details of a token.
type Details struct {
	Name     string // the name of the token, e.g., "USD Coin"
	Decimals int    // the power of ten to use when representing the "whole" unit of the token from its base value
}
