package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type vector struct {
	Name      string `json:"name"`
	SeedHex   string `json:"seed_hex"`
	PublicKey string `json:"public_key"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
	BodyB64   string `json:"body_b64"`
	Canonical string `json:"canonical"`
	SigB64    string `json:"signature_b64"`
}

func TestGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../obelisk-agent/internal/auth/testdata/vectors.json")
	if err != nil {
		t.Skipf("vectors.json not found: %v", err)
	}

	var vecs []vector
	if err := json.Unmarshal(data, &vecs); err != nil {
		t.Fatal(err)
	}

	for _, v := range vecs {
		t.Run(v.Name, func(t *testing.T) {
			seed, err := hex.DecodeString(v.SeedHex)
			if err != nil {
				t.Fatal(err)
			}
			priv := ed25519.NewKeyFromSeed(seed)

			pub := priv.Public().(ed25519.PublicKey)
			gotPub := "obk1_" + base64.StdEncoding.EncodeToString(pub)
			if gotPub != v.PublicKey {
				t.Errorf("public key: got %q want %q", gotPub, v.PublicKey)
			}

			var body []byte
			if v.BodyB64 != "" {
				body, err = base64.StdEncoding.DecodeString(v.BodyB64)
				if err != nil {
					t.Fatal(err)
				}
			}

			timestamp := fmt.Sprintf("%d", v.Timestamp)
			bodyHash := BodyHash(body)
			canonical := strings.Join([]string{"v1", v.Method, v.Path, timestamp, v.Nonce, bodyHash}, "\n")

			if canonical != v.Canonical {
				t.Errorf("canonical:\ngot  %q\nwant %q", canonical, v.Canonical)
			}

			sig := ed25519.Sign(priv, []byte(canonical))
			gotSig := base64.StdEncoding.EncodeToString(sig)
			if gotSig != v.SigB64 {
				t.Errorf("signature: got %q want %q", gotSig, v.SigB64)
			}
		})
	}
}
