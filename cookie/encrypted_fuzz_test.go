package cookie

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestEncryptedCookie(t testing.TB, secret string) Cookie {

	t.Helper()

	c, err := NewCookie(context.Background(), "encrypted://?name=test&secret="+secret+"&salt=salt1234")

	if err != nil {
		t.Fatal(err)
	}

	return c
}

func TestEncryptedCookieRoundTripSecretLengths(t *testing.T) {

	for _, n := range []int{0, 1, 16, 24, 31, 33, 64} {

		if _, err := NewCookie(context.Background(), "encrypted://?name=test&secret="+strings.Repeat("k", n)+"&salt=salt1234"); err == nil {
			t.Errorf("secret of %d bytes should be rejected", n)
		}
	}

	for _, n := range []int{32} {

		c := newTestEncryptedCookie(t, strings.Repeat("k", n))

		for _, v := range []string{"", "x", "héllo wörld", strings.Repeat("z", 3000)} {

			rec := httptest.NewRecorder()

			if err := c.SetString(rec, v); err != nil {
				t.Fatalf("len %d: %v", n, err)
			}

			req := httptest.NewRequest("GET", "/", nil)

			for _, ck := range rec.Result().Cookies() {
				req.AddCookie(ck)
			}

			got, err := c.GetString(req)

			if err != nil || got != v {
				t.Fatalf("len %d: got %q, %v", n, got, err)
			}
		}
	}
}

// The same plaintext must never produce the same ciphertext (random nonce).
func TestEncryptedCookieNonceUnique(t *testing.T) {

	c := newTestEncryptedCookie(t, strings.Repeat("k", 32))

	seen := map[string]bool{}

	for i := 0; i < 50; i++ {

		rec := httptest.NewRecorder()
		c.SetString(rec, "same")

		v := rec.Result().Cookies()[0].Value

		if seen[v] {
			t.Fatal("ciphertext repeated")
		}

		seen[v] = true
	}
}

func TestEncryptedCookieTamper(t *testing.T) {

	c := newTestEncryptedCookie(t, strings.Repeat("k", 32))

	rec := httptest.NewRecorder()
	c.SetString(rec, "secret value")

	orig := rec.Result().Cookies()[0]

	for i := 0; i < len(orig.Value); i++ {

		b := []byte(orig.Value)
		b[i] ^= 1

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: orig.Name, Value: string(b)})

		if got, err := c.GetString(req); err == nil {
			t.Fatalf("flipping byte %d was accepted, got %q", i, got)
		}
	}

	// A cookie from a different key must not decrypt.
	other := newTestEncryptedCookie(t, strings.Repeat("j", 32))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(orig)

	if _, err := other.GetString(req); err == nil {
		t.Fatal("cookie decrypted with the wrong key")
	}
}

func FuzzEncryptedCookieGet(f *testing.F) {

	c := newTestEncryptedCookie(f, strings.Repeat("k", 32))

	rec := httptest.NewRecorder()
	c.SetString(rec, "seed")

	f.Add(rec.Result().Cookies()[0].Value)
	f.Add("")
	f.Add("AAAA")
	f.Add("!!!!")

	f.Fuzz(func(t *testing.T, v string) {

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "test", Value: v})

		// Must return an error or a value, never panic.
		c.GetString(req)
	})
}
