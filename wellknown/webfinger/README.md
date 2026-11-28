# go-http/v4/wellknown/webfinger

Go package defining types and constants for [WebFinger](https://www.rfc-editor.org/rfc/rfc7033) (RFC 7033) responses.

* `Endpoint` is the well-known path (`/.well-known/webfinger`).
* `ContentType` is the media type of responses (`application/jrd+json`).
* `Resource` is a JSON Resource Descriptor (JRD) and `Link` is a link relation in a `Resource`.

This package does not provide an HTTP handler.

## Example

```
package main

import (
	"encoding/json"
	"net/http"

	"github.com/aaronland/go-http/v4/wellknown/webfinger"
)

func handler(rsp http.ResponseWriter, req *http.Request) {

	r := webfinger.Resource{
		Subject: req.URL.Query().Get("resource"),
		Links: []webfinger.Link{
			{Rel: "self", Type: "application/activity+json", HRef: "https://example.com/users/alice"},
		},
	}

	rsp.Header().Set("Content-Type", webfinger.ContentType)
	json.NewEncoder(rsp).Encode(r)
}

func main() {
	http.HandleFunc(webfinger.Endpoint, handler)
	http.ListenAndServe("localhost:8080", nil)
}
```

Note that this example echoes the `resource` query parameter and does no validation. A real handler should check that it names a resource it knows about.
