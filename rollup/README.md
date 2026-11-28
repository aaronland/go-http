# go-http/v4/rollup

Go package providing HTTP handlers for "rolling up" (bundling and minifying) JavaScript and CSS assets.

Each handler reads an ordered list of files from an `fs.FS`, minifies them using [tdewolff/minify](https://github.com/tdewolff/minify), concatenates them and serves the result as a single response.

## Example

_Error handling omitted for brevity._

```
package main

import (
	"embed"
	"net/http"

	"github.com/aaronland/go-http/v4/rollup"
)

//go:embed *.js
var js_fs embed.FS

func main() {

	opts := &rollup.RollupJSHandlerOptions{
		FS: js_fs,
		Paths: map[string][]string{
			"app.rollup.js": []string{"vendor.js", "app.js"},
		},
	}

	h, _ := rollup.RollupJSHandler(opts)

	mux := http.NewServeMux()
	mux.Handle("/javascript/app.rollup.js", h)

	http.ListenAndServe("localhost:8080", mux)
}
```

`RollupCSSHandler` works the same way using `RollupCSSHandlerOptions`.

## Behaviour

* The file served is selected using the last element of the request path (so `/javascript/app.rollup.js` selects `app.rollup.js`) and must be a key in `Paths`. Anything else is an HTTP 404.
* Only `GET` and `HEAD` requests are served. Anything else is an HTTP 405.
* Output is built in memory and is only sent if every file is read and minified successfully. Otherwise the response is an HTTP 500 and details are written to the log, not to the client.
* Handlers return an error on creation if the options or `FS` are missing or if any path in `Paths` does not exist in `FS`.
* JavaScript files are joined with a `;`.
* By default files are read and minified on every request. Set the `Cache` option to build each file once and serve it from memory with an `ETag` (and respond to `If-None-Match` with an HTTP 304). Changes to the files are not picked up while caching is enabled.

## See also

* https://github.com/tdewolff/minify