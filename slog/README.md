# go-http/v4/slog

Go package for deriving [log/slog](https://pkg.go.dev/log/slog) loggers decorated with details of an HTTP request.

## Functions

* `LoggerWithRequest(req, logger)` returns a `*slog.Logger` with the following keys set: `method`, `user agent`, `accept`, `path`, `remote addr` and `user ip`. If `logger` is `nil` the default `slog` logger is used. In that case each key is only added the first time it is seen by the process.
* `ReadUserIP(req)` returns the value of the `X-Real-Ip` header, then the `X-Forwarded-For` header and finally the request's remote address.

Both headers can be set by the client, so treat `user ip` as informational unless you are behind a proxy that overwrites them.

## Example

```
package main

import (
	"net/http"

	"github.com/aaronland/go-http/v4/slog"
)

func handler(rsp http.ResponseWriter, req *http.Request) {

	logger := slog.LoggerWithRequest(req, nil)
	logger.Info("Hello")
}
```
