# go-http/v4/static

Opinionated Go package for serving embedded static assets and referencing them from the HTML of web applications.

## Functions

* `StaticAssetsHandler(fs)` and `StaticAssetsHandlerWithPrefix(fs, prefix)` return an `http.Handler` that serves the files in an `fs.FS`. The prefix is removed from the request path before looking up a file.
* `AppendStaticAssetHandlers(mux, fs)` and `AppendStaticAssetHandlersWithPrefix(mux, fs, prefix)` register that handler on an `http.ServeMux` for each file in the `fs.FS`.
* `AppendResourcesHandler(next, opts)` and `AppendResourcesHandlerWithPrefix(next, opts, prefix)` are middleware that rewrite the HTML produced by `next` to include the JavaScript and CSS URIs and `data-*` body attributes defined in `ResourcesOptions` (see the [rewrite](../rewrite) package). Set `AppendJavaScriptAtEOF` to add the JavaScript at the end of the document rather than in `<head>`.

## Example

_Error handling omitted for brevity._

```
package main

import (
	"embed"
	"net/http"

	"github.com/aaronland/go-http/v4/static"
)

//go:embed css/*.css javascript/*.js
var static_fs embed.FS

func main() {

	mux := http.NewServeMux()
	static.AppendStaticAssetHandlersWithPrefix(mux, static_fs, "/static")

	opts := static.DefaultResourcesOptions()
	opts.CSS = append(opts.CSS, "/css/app.css")
	opts.JS = append(opts.JS, "/javascript/app.js")

	handler := static.AppendResourcesHandlerWithPrefix(mux, opts, "/static")
	http.ListenAndServe("localhost:8080", handler)
}
```

## See also

* [bootstrap](../bootstrap), which uses this package to serve embedded Bootstrap assets.
* [rollup](../rollup)

