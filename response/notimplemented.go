package response

import (
	"net/http"
)

// NotImplementedHandler returns an `http.Handler` that responds with an HTTP 501 (Not Implemented) error.
func NotImplementedHandler() http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		http.Error(rsp, "Not Implemented", http.StatusNotImplemented)
	}

	return http.HandlerFunc(fn)
}
