package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ImportOpenCodeCredentials sends credentials only in the authenticated request
// body. Peer response bodies are deliberately excluded from diagnostics.
func (c *Client) ImportOpenCodeCredentials(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/agent/opencode/credentials", bytes.NewReader(data))
	if err != nil {
		return errors.New("could not prepare OpenCode credential import")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.New("could not deliver OpenCode credentials")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return errors.New("OpenCode credential import failed; ensure agentctl and OpenCode support credential provisioning")
	}
	var result struct {
		Success bool `json:"success"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1024)).Decode(&result) != nil || !result.Success {
		return errors.New("OpenCode credential import was not acknowledged")
	}
	return nil
}
