# go-http/v4/sanitize

Go package for extracting, sanitising and type-converting parameters from HTTP requests.

Values can be read from URL query parameters (`Get*`), POST form bodies (`Post*`) or request headers (`Header*`). The `Request*` functions read from the form body for `POST` requests and from the query string for everything else. Each has a `String`, `Int`, `Int64`, `Float64` and `Bool` variant (headers: `String`, `Int64` and `Bool`) and, for query and form values, a `*Multi` variant that returns a slice for repeated parameters.

String values are sanitised using [whosonfirst/go-sanitize](https://github.com/whosonfirst/go-sanitize) before they are returned or converted. This strips control characters but does _not_ HTML-escape values, so use an escaping template engine when writing them to HTML. Missing parameters return a zero value (or an empty slice) without an error. A value that can not be converted returns an error.

## Example

```
package main

import (
	"net/http"

	"github.com/aaronland/go-http/v4/sanitize"
)

func handler(rsp http.ResponseWriter, req *http.Request) {

	q, err := sanitize.GetString(req, "q")

	if err != nil {
		http.Error(rsp, "Bad request", http.StatusBadRequest)
		return
	}

	page, err := sanitize.GetInt(req, "page")

	if err != nil {
		http.Error(rsp, "Bad request", http.StatusBadRequest)
		return
	}

	rsp.Write([]byte(q))
	_ = page
}
```

## See also

* https://github.com/whosonfirst/go-sanitize