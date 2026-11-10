package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"statemachine/transport"
	"time"
)

type HttpTransport struct {
	peers  map[string]string
	client *http.Client
}

func CreateNewHttpTransport(peers map[string]string, timeout time.Duration) (*HttpTransport, error) {
	if peers == nil {
		return &HttpTransport{}, fmt.Errorf("List of peers cannot be nil")
	}

	if timeout < 1 {
		return &HttpTransport{}, fmt.Errorf("Timeouts cannot be less than 1")
	}

	return &HttpTransport{
		peers:  peers,
		client: &http.Client{Timeout: timeout},
	}, nil
}

func (ht *HttpTransport) SendAppendEntries(ctx context.Context, peerId string, req transport.AppendEntriesRequest) (transport.AppendEntriesResponse, error) {

	var resp transport.AppendEntriesResponse

	addr, ok := ht.peers[peerId]

	if !ok {
		return resp, fmt.Errorf("Unknown peer with peerId: %s", peerId)
	}

	body, err := json.Marshal(req)

	if err != nil {
		return resp, fmt.Errorf("Error marshalling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader(body))

	if err != nil {
		return resp, fmt.Errorf("Error building http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := ht.client.Do(httpReq)

	if err != nil {
		return resp, fmt.Errorf("Error executing the request: %w", err)
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("Peer with peerId %s, returned %d", peerId, httpResp.StatusCode)
	}

	if err := json.NewDecoder((httpResp.Body)).Decode((&resp)); err != nil {
		return resp, fmt.Errorf("Error decoding http response: %w", err)
	}

	return resp, nil

}
