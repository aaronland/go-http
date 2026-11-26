package response

import (
	"net/http"
)

// NullHandler returns an `http.Handler` that does nothing, resulting in an empty HTTP 200 response.
func NullHandler() http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {
		return
	}

	return http.HandlerFunc(fn)
}
