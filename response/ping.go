package response

import (
	"net/http"
)

// PingPongHandler returns an `http.Handler` equivalent to `PingHandler` with a response value of "PONG".
func PingPongHandler() (http.Handler, error) {
	return PingHandler("PONG")
}

// PingHandler returns an `http.Handler` that responds with an HTTP 204 (No Content) status. If 'response'
// is not empty it is sent in the `X-ping` response header.
func PingHandler(response string) (http.Handler, error) {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		rsp.Header().Set("Content-Type", "text/plain")

		if response != "" {
			rsp.Header().Set("X-ping", response)
		}

		rsp.WriteHeader(http.StatusNoContent)
	}

	h := http.HandlerFunc(fn)
	return h, nil
}
