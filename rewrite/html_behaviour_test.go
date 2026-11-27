package rewrite

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func noopRewrite(n *html.Node, w io.Writer) {}

func upperTitle(n *html.Node, w io.Writer) {
	if n.Type == html.TextNode {
		n.Data = strings.ToUpper(n.Data)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		upperTitle(c, w)
	}
}

func run(h http.Handler) *httptest.ResponseRecorder {
	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", "/", nil))
	return rsp
}

func TestRewriteHTMLHandlerRedirectKeepsStatus(t *testing.T) {

	for _, status := range []int{301, 302, 303, 307, 308} {

		next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
			rsp.Header().Set("Location", "/elsewhere")
			rsp.Header().Add("Set-Cookie", "a=1")
			rsp.Header().Add("Set-Cookie", "b=2")
			rsp.WriteHeader(status)
		})

		rsp := run(RewriteHTMLHandler(next, noopRewrite))

		if rsp.Code != status {
			t.Errorf("status %d became %d", status, rsp.Code)
		}

		if rsp.Header().Get("Location") != "/elsewhere" {
			t.Errorf("status %d: Location=%q", status, rsp.Header().Get("Location"))
		}

		if got := rsp.Header().Values("Set-Cookie"); len(got) != 2 {
			t.Errorf("status %d: expected both cookies to survive, got %v", status, got)
		}
	}
}

func TestRewriteHTMLHandlerNonHTMLPassesThrough(t *testing.T) {

	for _, ct := range []string{"application/json", "text/plain", "image/png"} {

		next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
			rsp.Header().Set("Content-Type", ct)
			rsp.WriteHeader(http.StatusCreated)
			rsp.Write([]byte("<b>raw</b>"))
		})

		rsp := run(RewriteHTMLHandler(next, upperTitle))

		if rsp.Code != http.StatusCreated {
			t.Errorf("%s: status %d", ct, rsp.Code)
		}

		if rsp.Body.String() != "<b>raw</b>" {
			t.Errorf("%s: body was modified: %q", ct, rsp.Body.String())
		}

		if rsp.Header().Get("Content-Type") != ct {
			t.Errorf("%s: content type became %q", ct, rsp.Header().Get("Content-Type"))
		}
	}
}

func TestRewriteHTMLHandlerContentTypeVariants(t *testing.T) {

	for _, ct := range []string{"text/html", "TEXT/HTML", "text/html; charset=utf-8", "Text/Html;charset=ISO-8859-1", ""} {

		next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {

			if ct != "" {
				rsp.Header().Set("Content-Type", ct)
			}

			rsp.Header().Set("X-Keep", "yes")
			rsp.Header().Set("Content-Length", "999")
			rsp.WriteHeader(http.StatusAccepted)
			rsp.Write([]byte("<html><body><p>hello</p></body></html>"))
		})

		rsp := run(RewriteHTMLHandler(next, upperTitle))

		if !strings.Contains(rsp.Body.String(), "HELLO") {
			t.Errorf("%q: HTML was not rewritten: %q", ct, rsp.Body.String())
		}

		if rsp.Code != http.StatusAccepted {
			t.Errorf("%q: status %d", ct, rsp.Code)
		}

		if rsp.Header().Get("X-Keep") != "yes" {
			t.Errorf("%q: header dropped", ct)
		}

		if rsp.Header().Get("Content-Length") == "999" {
			t.Errorf("%q: stale Content-Length retained", ct)
		}
	}
}

func TestRewriteHTMLHandlerEmptyBody(t *testing.T) {

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		rsp.Header().Set("Content-Type", "text/html")
	})

	rsp := run(RewriteHTMLHandler(next, noopRewrite))

	if rsp.Code != http.StatusOK {
		t.Fatalf("got %d", rsp.Code)
	}
}

func TestRewriteHTMLHandlerMalformedHTML(t *testing.T) {

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		rsp.Header().Set("Content-Type", "text/html")
		rsp.Write([]byte("<div><span>unclosed <b>tags"))
	})

	rsp := run(RewriteHTMLHandler(next, noopRewrite))

	if rsp.Code != http.StatusOK || !strings.Contains(rsp.Body.String(), "unclosed") {
		t.Fatalf("got %d %q", rsp.Code, rsp.Body.String())
	}
}
