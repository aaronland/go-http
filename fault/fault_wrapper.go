package fault

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

// FaultWrapper is a struct to make assigning `TemplatedFaultHandlerWrapper` instances easier.
type FaultWrapper struct {
	// logger is the *log.Logger instance that will be passed to the underlying `TemplatedFaultHandler`
	logger *log.Logger
	// template is the *template.Template instance that will be executed by the underlying `TemplatedFaultHandler`
	template *template.Template
}

// NewFaultWrapper creates a new `FaultWrapper` instance.
func NewFaultWrapper(logger *log.Logger, template *template.Template) *FaultWrapper {

	fw := &FaultWrapper{
		template: template,
		logger:   logger,
	}

	return fw
}

// HandleWithMux will wrap 'h' in a new `TemplatedFaultHandlerWrapper` middleware handler and then assign it to 'mux' with pattern 'uri'.
func (fw *FaultWrapper) HandleWithMux(mux *http.ServeMux, uri string, h http.Handler) {
	wr := TemplatedFaultHandlerWrapper(fw.logger, fw.template, h)
	mux.Handle(uri, wr)
}

// TemplatedFaultHandlerWrapper will return a middleware `http.Handler` that when invoked will serve 'h'
// and if the response status code is >= `http.StatusBadRequest` (400) will serve a new fault handler
// using 't' and 'l'.
func TemplatedFaultHandlerWrapper(l *log.Logger, t *template.Template, h http.Handler) http.Handler {

	opts := &FaultHandlerOptions{
		Logger:   l,
		Template: t,
		VarsFunc: defaultFaultHandlerVars,
	}

	fh := FaultHandlerWithOptions(opts)

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		iw := &interceptWriter{ResponseWriter: rsp}
		h.ServeHTTP(iw, req)

		if !iw.intercepted {
			return
		}

		// The inner handler's error response was discarded; remove headers describing its body.
		hdr := rsp.Header()
		hdr.Del("Content-Length")
		hdr.Del("Content-Encoding")
		hdr.Del("Etag")
		hdr.Del("Last-Modified")

		// If the inner handler recorded no error, report its status code.
		_, ok := req.Context().Value(statusContextKey).(int)

		if !ok {
			AssignError(req, fmt.Errorf("%s", http.StatusText(iw.status)), iw.status)
		}

		fh.ServeHTTP(rsp, req)
	}

	return http.HandlerFunc(fn)
}

// interceptWriter passes responses with a status below 400 through and discards the status and body of
// any others so that the fault handler can write the response instead.
type interceptWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	intercepted bool
}

func (w *interceptWriter) WriteHeader(status int) {

	if w.wroteHeader {
		return
	}

	w.wroteHeader = true
	w.status = status

	if status >= http.StatusBadRequest {
		w.intercepted = true
		return
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *interceptWriter) Write(b []byte) (int, error) {

	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if w.intercepted {
		return len(b), nil
	}

	return w.ResponseWriter.Write(b)
}

func (w *interceptWriter) Flush() {

	if w.intercepted {
		return
	}

	f, ok := w.ResponseWriter.(http.Flusher)

	if ok {
		w.wroteHeader = true
		f.Flush()
	}
}
