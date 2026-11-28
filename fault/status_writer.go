package fault

// Cloned from https://raw.githubusercontent.com/joegasewicz/status-writer/master/status_writer.go

import (
	"net/http"
)

// StatusWriter type adds a `Status` property to Go's `http.ResponseWriter` type.
type StatusWriter struct {
	http.ResponseWriter
	// Status is the status code that was sent to the client: the first one written, or `http.StatusOK`.
	Status      int
	wroteHeader bool
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

// WriteHeader records 'status' (only the first call counts, as that is what the client sees) and then
// calls the underlying `http.ResponseWriter`'s `WriteHeader` method.
func (w *StatusWriter) WriteHeader(status int) {

	if !w.wroteHeader {
		w.Status = status
		w.wroteHeader = true
	}

	w.ResponseWriter.WriteHeader(status)
}

// Write marks the status as sent (an implicit 200) and writes to the underlying `http.ResponseWriter`.
func (w *StatusWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

// Flush calls the underlying writer's `Flush` method, if it has one.
func (w *StatusWriter) Flush() {

	f, ok := w.ResponseWriter.(http.Flusher)

	if ok {
		w.wroteHeader = true
		f.Flush()
	}
}

// Unwrap returns the underlying `http.ResponseWriter` for use with `http.ResponseController`.
func (w *StatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
