package oauth

import (
	"strings"
	"testing"
	"time"
)

func TestTokenRoundTripAndTamper(t *testing.T) {
	s := signer{key: []byte("0123456789abcdef0123456789abcdef")}
	now := time.Unix(1_800_000_000, 0)
	tok, err := s.sign(claims{Typ: typAccess, Sub: 7, Aud: "https://x/mcp", ClientID: "c", Exp: now.Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if c, err := s.verify(tok, typAccess, now); err != nil || c.Sub != 7 {
		t.Fatalf("verify: %v %+v", err, c)
	}
	if _, err := s.verify(tok, typRefresh, now); err == nil {
		t.Error("access token accepted as refresh")
	}
	if _, err := s.verify(tok, typAccess, now.Add(time.Hour)); err == nil {
		t.Error("expired token accepted")
	}
	other := signer{key: []byte("fedcba9876543210fedcba9876543210")}
	if _, err := other.verify(tok, typAccess, now); err == nil {
		t.Error("token accepted under a rotated key")
	}
	i := strings.LastIndexByte(tok, '.')
	payload := tok[len(tokenPrefix):i]
	flipped := []byte(payload)
	flipped[5] ^= 1
	if _, err := s.verify(tokenPrefix+string(flipped)+tok[i:], typAccess, now); err == nil {
		t.Error("tampered payload accepted")
	}
}

func TestPKCE(t *testing.T) {
	// RFC 7636 appendix B.
	if !pkceOK("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM") {
		t.Fatal("RFC 7636 example fails")
	}
	if pkceOK("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXl", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM") {
		t.Fatal("wrong verifier accepted")
	}
}
