# go-http/v4/response

Go package providing simple `http.Handler` implementations for common responses.

## Handlers

| Handler | Description |
| --- | --- |
| `NullHandler()` | Does nothing, resulting in an empty HTTP 200 response. |
| `NotImplementedHandler()` | Responds with an HTTP 501 (Not Implemented) error. |
| `DisabledHandler(disabled, next)` | Middleware that responds with an HTTP 503 (Service Unavailable) error if `disabled` is true, otherwise it serves `next`. |
| `PingHandler(response)` | Responds with an HTTP 204 (No Content). If `response` is not empty it is sent in the `X-ping` header. |
| `PingPongHandler()` | Like `PingHandler` with a response value of `PONG`. Useful for health checks. |

## Example

```
package main

import (
	"net/http"

	"github.com/aaronland/go-http/v4/response"
)

func main() {

	ping_handler, _ := response.PingPongHandler()

	mux := http.NewServeMux()
	mux.Handle("/ping", ping_handler)
	mux.Handle("/beta", response.DisabledHandler(true, response.NullHandler()))

	http.ListenAndServe("localhost:8080", mux)
}
```
