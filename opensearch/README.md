# go-http/v4/opensearch

Go package for publishing [OpenSearch description documents](https://github.com/dewitt/opensearch) and advertising them to browsers.

## Documentation

[Documentation is incomplete](https://pkg.go.dev/github.com/aaronland/go-http/v4/opensearch).

## What it does

* `OpenSearchDescription` (and the related `OpenSearchImage`, `OpenSearchURL` and `OpenSearchURLParameter` types) model an OpenSearch description document and can be marshaled to XML.
* `BasicOpenSearchDescription` creates a description from a small set of options (name, description, query parameter, search template, search form and icon).
* `OpenSearchHandler` returns an `http.Handler` that serves a description document with the correct content type.
* `AppendOpenSearchPluginsHandler` is middleware that adds a `<link rel="search">` element to the `<head>` of HTML responses so that browsers can discover the search engines.

## Example

_Error handling omitted for brevity._

```
package main

import (
	"net/http"

	"github.com/aaronland/go-http/v4/opensearch"
)

func main() {

	desc, _ := opensearch.BasicOpenSearchDescription(&opensearch.BasicOpenSearchDescriptionOptions{
		Name:           "Example",
		Description:    "Search example.com",
		QueryParameter: "q",
		SearchTemplate: "https://example.com/search?q={searchTerms}",
		SearchForm:     "https://example.com/search",
		ImageURI:       "data:image/png;base64,...",
	})

	desc_handler, _ := opensearch.OpenSearchHandler(desc)

	mux := http.NewServeMux()
	mux.Handle("/opensearch.xml", desc_handler)

	plugins := map[string]*opensearch.OpenSearchDescription{
		"/opensearch.xml": desc,
	}

	handler := opensearch.AppendOpenSearchPluginsHandler(mux, plugins)
	http.ListenAndServe("localhost:8080", handler)
}
```

## See also

* [rewrite](../rewrite), which is used to modify the HTML.
