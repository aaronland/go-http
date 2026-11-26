package fault

// Cloned from https://raw.githubusercontent.com/joegasewicz/status-writer/master/status_writer.go

import (
	"net/http"
)

// StatusWriter type adds a `Status` property to Go's `http.ResponseWriter` type.
type StatusWriter struct {
	http.ResponseWriter
	Status int
}

// NewStatusWriter returns a new `StatusWriter` wrapping 'w'. It is intended for middleware
// (such as a logging function) that requires access to the status code written to
// `http.ResponseWriter`. The default status is `http.StatusOK`.
//
//			func MyMiddleware(h http.HandleFunc) http.HandlerFunc {
//					return func(w http.ResponseWriter, r *http.Request) {
//	       			sw := fault.NewStatusWriter(w) // Here we override http.ResponseWriter's `WriteHeader` function
//						h(sw, r)
//	       			statusCode := sw.Status // Now work with the status code
//	   			}
//			}
func NewStatusWriter(w http.ResponseWriter) *StatusWriter {
	return &StatusWriter{
		ResponseWriter: w,
		Status:         http.StatusOK,
	}
}

// WriteHeader records 'status' and then calls the underlying `http.ResponseWriter`'s `WriteHeader` method.
func (w *StatusWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}
