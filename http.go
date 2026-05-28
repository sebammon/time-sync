package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpClient struct {
	base    string
	headers map[string]string
	client  *http.Client
}

func newHTTPClient(baseURL string, headers map[string]string) *httpClient {
	return &httpClient{
		base:    baseURL,
		headers: headers,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *httpClient) get(path string, out any) error {
	return h.do("GET", path, nil, out)
}

func (h *httpClient) post(path string, body, out any) error {
	return h.do("POST", path, body, out)
}

func (h *httpClient) put(path string, body, out any) error {
	return h.do("PUT", path, body, out)
}

func (h *httpClient) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, h.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s %s: %d %s: %s", method, path, resp.StatusCode, resp.Status, string(b))
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
