package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "obelisk"), nil
}

func privKeyPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "id_ed25519"), nil
}

// Generate creates a new ED25519 keypair. Returns an error if a key already
// exists unless force is true.
func Generate(force bool) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := privKeyPath()
	if err != nil {
		return err
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return errors.New("key already exists; use --force to overwrite")
		}
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}

	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return err
	}

	pub := priv.Public().(ed25519.PublicKey)
	pubStr := "obk1_" + base64.StdEncoding.EncodeToString(pub)
	pubPath := path + ".pub"
	return os.WriteFile(pubPath, []byte(pubStr+"\n"), 0644)
}

// Load reads the private key from disk.
func Load() (ed25519.PrivateKey, error) {
	path, err := privKeyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("no identity key found; run `obelisk identity` to create one")
		}
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid key file: not PEM encoded")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("key file does not contain an ED25519 key")
	}
	return priv, nil
}

// LoadOrGenerate loads the existing keypair or generates one if none exists.
func LoadOrGenerate() (ed25519.PrivateKey, error) {
	priv, err := Load()
	if err == nil {
		return priv, nil
	}
	if err := Generate(false); err != nil {
		return nil, err
	}
	return Load()
}

// PublicKeyString returns the public key in "obk1_<base64>" format.
func PublicKeyString() (string, error) {
	priv, err := LoadOrGenerate()
	if err != nil {
		return "", err
	}
	pub := priv.Public().(ed25519.PublicKey)
	return "obk1_" + base64.StdEncoding.EncodeToString(pub), nil
}

// Fingerprint returns the "SHA256:<base64url-nopad>" fingerprint of the public key.
func Fingerprint() (string, error) {
	priv, err := LoadOrGenerate()
	if err != nil {
		return "", err
	}
	pub := priv.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(pub)
	return "SHA256:" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// Sign builds the canonical string and returns a base64-encoded ED25519 signature.
func Sign(method, path, timestamp, nonce, bodyHash string) (string, error) {
	priv, err := Load()
	if err != nil {
		return "", err
	}
	canonical := strings.Join([]string{"v1", method, path, timestamp, nonce, bodyHash}, "\n")
	sig := ed25519.Sign(priv, []byte(canonical))
	return base64.StdEncoding.EncodeToString(sig), nil
}

// BodyHash returns the lowercase hex SHA256 of body (pass nil or empty for GETs).
func BodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
