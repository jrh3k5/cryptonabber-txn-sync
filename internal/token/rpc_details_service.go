package token

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

// rpcDetailsService implements DetailsService by calling an RPC node.
type rpcDetailsService struct {
	client *http.Client
	rpcURL string
}

// NewRPCDetailsService returns a DetailsService that uses the provided HTTP client
// and RPC node URL to perform JSON-RPC calls.
func NewRPCDetailsService(client *http.Client, rpcURL string) DetailsService {
	return &rpcDetailsService{client: client, rpcURL: rpcURL}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Result  string    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

// GetTokenDetails fetches the token decimals by calling the `decimals()` ERC20 method
// using `eth_call` on the RPC node. If no result is returned, it returns (nil, nil).
func (r *rpcDetailsService) GetTokenDetails(ctx context.Context, contractAddress string) (*Details, error) {
	if r.client == nil {
		return nil, errors.New("http client is nil")
	}
	// decimals() selector
	data := "0x313ce567"

	// prepare params: call object and block param
	callObj := map[string]string{
		"to":   contractAddress,
		"data": data,
	}

	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_call",
		Params:  []any{callObj, "latest"},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.rpcURL, strings.NewReader(string(b)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rpc call: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	res := rpcResp.Result
	if res == "" || res == "0x" {
		// no data found
		return nil, nil
	}

	// strip 0x
	hexStr := strings.TrimPrefix(res, "0x")
	if len(hexStr) == 0 {
		return nil, nil
	}
	// ensure even length for hex decode
	if len(hexStr)%2 == 1 {
		hexStr = "0" + hexStr
	}

	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode hex result: %w", err)
	}

	bi := new(big.Int).SetBytes(decoded)
	if bi.BitLen() == 0 {
		return nil, nil
	}

	// convert to int safely
	if bi.Cmp(big.NewInt(int64(^uint(0)>>1))) == 1 { // bigger than max int
		return nil, fmt.Errorf("decimals value too large: %s", bi.String())
	}
	decimals := int(bi.Int64())

	return &Details{Decimals: decimals}, nil
}
