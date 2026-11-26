package opensearch

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"io"
	"net/http"

	"github.com/aaronland/go-http/v4/rewrite"
)

// OpenSearchHandler returns an `http.Handler` that serves 'desc' as an XML OpenSearch description document.
func OpenSearchHandler(desc *OpenSearchDescription) (http.Handler, error) {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		body, err := desc.Marshal()

		if err != nil {
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		rsp.Header().Set("Content-Type", OPENSEARCH_CONTENT_TYPE)
		rsp.Write(body)
	}

	h := http.HandlerFunc(fn)
	return h, nil
}

// AppendOpenSearchPluginsHandler returns a middleware `http.Handler` that serves 'next' with a `<link rel="search">`
// element added to the `<head>` of HTML responses for each OpenSearch description in 'plugins'. The keys of 'plugins'
// are the URIs of the corresponding description documents.
func AppendOpenSearchPluginsHandler(next http.Handler, plugins map[string]*OpenSearchDescription) http.Handler {

	var cb rewrite.RewriteHTMLFunc

	cb = func(n *html.Node, w io.Writer) {

		if n.Type == html.ElementNode && n.Data == "head" {

			for uri, d := range plugins {

				link_rel := html.Attribute{"", "rel", "search"}
				link_type := html.Attribute{"", "type", OPENSEARCH_CONTENT_TYPE}
				link_href := html.Attribute{"", "href", uri}
				link_title := html.Attribute{"", "title", d.ShortName}

				link := html.Node{
					Type:      html.ElementNode,
					DataAtom:  atom.Link,
					Data:      "link",
					Namespace: "",
					Attr: []html.Attribute{
						link_rel,
						link_type,
						link_href,
						link_title,
					},
				}

				n.AppendChild(&link)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			cb(c, w)
		}
	}

	return rewrite.RewriteHTMLHandler(next, cb)
}
