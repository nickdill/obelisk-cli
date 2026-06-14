package client

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nickdill/obelisk/internal/identity"
)

// Client makes signed HTTP requests to an obelisk-agent.
// BaseURL is the root of the Obelisk server, e.g. "https://myhost.com".
// Requests are sent to {BaseURL}/_obelisk{agentPath} but signed with {agentPath}.
type Client struct {
	BaseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 30 * time.Minute,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
	}
}

func (c *Client) Get(agentPath string) (*http.Response, error) {
	return c.do("GET", agentPath, nil)
}

func (c *Client) Post(agentPath string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.do("POST", agentPath, data)
}

func (c *Client) Delete(agentPath string) (*http.Response, error) {
	return c.do("DELETE", agentPath, nil)
}

func (c *Client) do(method, agentPath string, body []byte) (*http.Response, error) {
	if err := c.enforceHTTPS(); err != nil {
		return nil, err
	}

	pubKey, err := identity.PublicKeyString()
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	bodyHash := identity.BodyHash(body)

	sig, err := identity.Sign(method, agentPath, timestamp, nonce, bodyHash)
	if err != nil {
		return nil, fmt.Errorf("signing request: %w", err)
	}

	fullURL := c.BaseURL + "/_obelisk" + agentPath
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Obelisk-Key", pubKey)
	req.Header.Set("X-Obelisk-Timestamp", timestamp)
	req.Header.Set("X-Obelisk-Nonce", nonce)
	req.Header.Set("X-Obelisk-Signature", sig)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	if err := c.handleAuthErrors(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// handleAuthErrors converts well-known 401/403 responses into actionable messages.
func (c *Client) handleAuthErrors(resp *http.Response) error {
	if resp.StatusCode != 401 && resp.StatusCode != 403 {
		return nil
	}

	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	data, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(data, &body)

	switch body.Error {
	case "unknown_key":
		pubKey, _ := identity.PublicKeyString()
		fp, _ := identity.Fingerprint()
		return fmt.Errorf(
			"this key is not authorized on the server\n\n"+
				"  Public key:  %s\n"+
				"  Fingerprint: %s\n\n"+
				"Ask a server admin to run:\n"+
				"  obelisk allow %s --server <name>",
			pubKey, fp, pubKey,
		)
	case "stale_timestamp":
		return fmt.Errorf("clock skew detected — your system clock may be off\n(server says: %s)", body.Message)
	case "bad_signature":
		return fmt.Errorf("signature rejected by server (bad_signature)")
	case "replay":
		return fmt.Errorf("request rejected as a replay — try again")
	default:
		if body.Message != "" {
			return fmt.Errorf("auth error (%d): %s", resp.StatusCode, body.Message)
		}
		return fmt.Errorf("auth error (%d)", resp.StatusCode)
	}
}

func (c *Client) enforceHTTPS() error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1") {
		return nil
	}
	return fmt.Errorf("insecure connection refused: use https:// for %s", c.BaseURL)
}

func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// DecodeJSON is a helper for callers to decode a successful response body.
func DecodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dst)
}
