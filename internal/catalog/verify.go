package catalog

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func hexSum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// verifyFeed checks feed.json.sig ({"kid","sig"}): sig is the Ed25519 signature of the exact bytes of feed.json, in
// standard base64, by the key the signature names.
func verifyFeed(key Key, feed, sig []byte) error {
	var s struct {
		Kid string `json:"kid"`
		Sig string `json:"sig"`
	}
	if err := json.Unmarshal(sig, &s); err != nil {
		return fmt.Errorf("feed.json.sig: %w", err)
	}
	if s.Kid != key.Kid {
		return fmt.Errorf("feed.json.sig: kid %q, want %q", s.Kid, key.Kid)
	}
	raw, err := base64.StdEncoding.DecodeString(s.Sig)
	if err != nil || !ed25519.Verify(key.Pub, feed, raw) {
		return errors.New("feed.json.sig does not verify")
	}
	return nil
}

// verifyJWS checks signatures/catalog.jws, a compact JWS signed with EdDSA by the catalog key, and returns its claims.
func verifyJWS(key Key, tok string) (Claims, error) {
	var c Claims
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return c, errors.New("catalog.jws: not a compact JWS")
	}
	hb, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	pb, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	sig, err3 := base64.RawURLEncoding.DecodeString(parts[2])
	if err := errors.Join(err1, err2, err3); err != nil {
		return c, fmt.Errorf("catalog.jws: %w", err)
	}
	var h struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(hb, &h); err != nil || h.Alg != "EdDSA" || h.Kid != key.Kid {
		return c, fmt.Errorf("catalog.jws: header %s", hb)
	}
	if !ed25519.Verify(key.Pub, []byte(parts[0]+"."+parts[1]), sig) {
		return c, errors.New("catalog.jws does not verify")
	}
	if err := json.Unmarshal(pb, &c); err != nil {
		return c, fmt.Errorf("catalog.jws claims: %w", err)
	}
	return c, nil
}
