package cookie

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEncryptedCookieRoundTrip(t *testing.T) {

	ctx := context.Background()

	uri, err := NewRandomEncryptedCookieURI("test")

	if err != nil {
		t.Fatal(err)
	}

	c, err := NewCookie(ctx, uri)

	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()

	if err := c.SetString(rec, "hello"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)

	for _, ck := range rec.Result().Cookies() {
		req.AddCookie(ck)
	}

	v, err := c.GetString(req)

	if err != nil {
		t.Fatal(err)
	}

	if v != "hello" {
		t.Fatalf("Unexpected value '%s'", v)
	}
}

func TestEncryptedCookieMalformed(t *testing.T) {

	ctx := context.Background()

	uri, _ := NewRandomEncryptedCookieURI("test")
	c, err := NewCookie(ctx, uri)

	if err != nil {
		t.Fatal(err)
	}

	for _, v := range []string{"", "AAAA", "not base64!", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "test", Value: v})

		if _, err := c.GetString(req); err == nil {
			t.Fatalf("Expected error for '%s'", v)
		}
	}
}

func TestEncryptedCookieShortSecret(t *testing.T) {

	_, err := NewCookie(context.Background(), "encrypted://?name=x&secret=short&salt=y")

	if err == nil {
		t.Fatal("Expected error for short secret")
	}
}
