package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Tokens are "mcp1.<payload>.<mac>": base64url JSON claims and an
// HMAC-SHA256 over "mcp1.<payload>". Nothing is stored, so a restart keeps
// Claude connected; rotating MCP_TOKEN_SIGNING_KEY revokes every token.
const tokenPrefix = "mcp1."

const (
	typAccess  = "access"
	typRefresh = "refresh"
)

var errBadToken = errors.New("invalid token")

type claims struct {
	Typ      string `json:"typ"`
	Sub      int64  `json:"sub"`
	Aud      string `json:"aud"`
	ClientID string `json:"cid"`
	Scope    string `json:"scope"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
	JTI      string `json:"jti"`
}

type signer struct{ key []byte }

func (s signer) sign(c claims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := tokenPrefix + base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + base64.RawURLEncoding.EncodeToString(s.mac(body)), nil
}

func (s signer) mac(body string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(body))
	return m.Sum(nil)
}

// verify checks the signature, type and expiry. Callers check aud, cid, sub.
func (s signer) verify(tok, typ string, now time.Time) (claims, error) {
	if len(tok) > 2048 || !strings.HasPrefix(tok, tokenPrefix) {
		return claims{}, errBadToken
	}
	i := strings.LastIndexByte(tok, '.')
	if i <= len(tokenPrefix) {
		return claims{}, errBadToken
	}
	body, sig := tok[:i], tok[i+1:]
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac(body)) {
		return claims{}, errBadToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(body[len(tokenPrefix):])
	if err != nil {
		return claims{}, errBadToken
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil || c.Typ != typ || now.Unix() >= c.Exp {
		return claims{}, errBadToken
	}
	return c, nil
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// TokenID is a short, non-reversible handle for logs.
func TokenID(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:6])
}
