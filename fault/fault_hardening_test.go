package fault

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

var quiet = log.New(io.Discard, "", 0)

func newTemplate(t *testing.T, s string) *template.Template {

	tpl, err := template.New("t").Parse(s)

	if err != nil {
		t.Fatalf("Failed to parse template, %v", err)
	}

	return tpl
}

func TestTemplatedHandlerSetsStatusAndHeaders(t *testing.T) {

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, errors.New("boom"), http.StatusNotFound)

	rec := httptest.NewRecorder()
	TemplatedFaultHandler(quiet, newTemplate(t, "ERR {{.Status}} {{.Error}}")).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Unexpected status %d", rec.Code)
	}

	if rec.Body.String() != "ERR 404 boom" {
		t.Fatalf("Unexpected body %q", rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Unexpected content type %q", rec.Header().Get("Content-Type"))
	}

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("Missing nosniff")
	}
}

func TestInvalidStatusBecomes500(t *testing.T) {

	for _, status := range []int{0, 99, 1000, -1} {

		req := httptest.NewRequest("GET", "/", nil)
		AssignError(req, errors.New("x"), status)

		rec := httptest.NewRecorder()
		FaultHandler(quiet).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("Status %d: got %d", status, rec.Code)
		}
	}
}

func TestNilOptionsDoNotPanic(t *testing.T) {

	handlers := []http.Handler{
		FaultHandler(nil),
		FaultHandlerWithOptions(nil),
		FaultHandlerWithOptions(&FaultHandlerOptions{Template: newTemplate(t, "{{.Status}}")}),
	}

	for _, h := range handlers {

		req := httptest.NewRequest("GET", "/", nil)
		AssignError(req, nil, 500)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != 500 {
			t.Fatalf("Unexpected status %d", rec.Code)
		}
	}
}

func TestRetrieveErrorWrongTypes(t *testing.T) {

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, errors.New("x"), 400)

	// Legacy keys carrying wrong types must not panic.
	req2 := httptest.NewRequest("GET", "/", nil)
	ctx := req2.Context()
	ctx = context.WithValue(ctx, ErrorKey, "not an error")
	ctx = context.WithValue(ctx, StatusKey, "not an int")
	req2 = req2.WithContext(ctx)

	status, err := RetrieveError(req2)

	if status != 500 || err == nil {
		t.Fatalf("Unexpected %d, %v", status, err)
	}
}

func TestLegacyKeysAreAssigned(t *testing.T) {

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, errors.New("x"), 418)

	if req.Context().Value(StatusKey) != 418 {
		t.Fatal("Legacy status key not assigned")
	}
}

func TestVarsValidation(t *testing.T) {

	type badStatus struct {
		Status string
		Error  error
	}

	type badError struct {
		Status int
		Error  string
	}

	if ImplementsFaultHandlerVars(nil) {
		t.Fatal("nil should not implement")
	}

	if ImplementsFaultHandlerVars(&badStatus{}) || ImplementsFaultHandlerVars(&badError{}) {
		t.Fatal("wrongly typed fields should not implement")
	}

	if !ImplementsFaultHandlerVars(&FaultHandlerVars{}) {
		t.Fatal("FaultHandlerVars should implement")
	}

	for _, v := range []any{nil, &badStatus{}, &badError{}, badStatus{}} {

		v := v

		h := FaultHandlerWithOptions(&FaultHandlerOptions{
			Logger:   quiet,
			Template: newTemplate(t, "x"),
			VarsFunc: func() any { return v },
		})

		req := httptest.NewRequest("GET", "/", nil)
		AssignError(req, errors.New("x"), 404)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != 404 || !strings.Contains(rec.Body.String(), "problem completing") {
			t.Fatalf("Expected fallback response, got %d %q", rec.Code, rec.Body.String())
		}
	}
}

type setterVars struct {
	status int
	err    error
}

func (s *setterVars) SetFaultVars(status int, err error) {
	s.status = status
	s.err = err
}

func (s *setterVars) Summary() string { return s.err.Error() }

func TestVarsSetter(t *testing.T) {

	h := FaultHandlerWithOptions(&FaultHandlerOptions{
		Logger:   quiet,
		Template: newTemplate(t, "{{.Summary}}"),
		VarsFunc: func() any { return &setterVars{} },
	})

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, errors.New("hello"), 400)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Body.String() != "hello" {
		t.Fatalf("Unexpected body %q", rec.Body.String())
	}
}

func TestSharedVarsAreNotRaced(t *testing.T) {

	type shared struct {
		Status int
		Error  error
	}

	s := &shared{}

	h := FaultHandlerWithOptions(&FaultHandlerOptions{
		Logger:   quiet,
		Template: newTemplate(t, "{{.Status}}"),
		VarsFunc: func() any { return s },
	})

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {

		wg.Add(1)

		go func(status int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/", nil)
			AssignError(req, errors.New("x"), status)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
		}(400 + i)
	}

	wg.Wait()

	if s.Status != 0 {
		t.Fatal("Shared vars were modified")
	}
}

func TestTemplateFailureSendsNoPartialOutput(t *testing.T) {

	tpl := newTemplate(t, "partial {{.Missing.Field}}")

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, errors.New("x"), 500)

	rec := httptest.NewRecorder()
	TemplatedFaultHandler(quiet, tpl).ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "partial") {
		t.Fatalf("Partial output sent: %q", rec.Body.String())
	}

	if rec.Code != 500 {
		t.Fatalf("Unexpected status %d", rec.Code)
	}
}

func TestGenericPublicErrors(t *testing.T) {

	opts := &FaultHandlerOptions{
		Logger:              quiet,
		Template:            newTemplate(t, "{{.Error}}"),
		GenericPublicErrors: true,
	}

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, errors.New("secret db password"), 500)

	rec := httptest.NewRecorder()
	FaultHandlerWithOptions(opts).ServeHTTP(rec, req)

	if rec.Body.String() != "Internal Server Error" {
		t.Fatalf("Unexpected body %q", rec.Body.String())
	}
}

type publicPrivate struct{}

func (publicPrivate) Error() string  { return "both" }
func (publicPrivate) Public() error  { return errors.New("public") }
func (publicPrivate) Private() error { return errors.New("private") }

func TestFaultErrorStillPublicWithGeneric(t *testing.T) {

	var logbuf bytes.Buffer

	opts := &FaultHandlerOptions{
		Logger:              log.New(&logbuf, "", 0),
		Template:            newTemplate(t, "{{.Error}}"),
		GenericPublicErrors: true,
	}

	req := httptest.NewRequest("GET", "/", nil)
	AssignError(req, publicPrivate{}, 400)

	rec := httptest.NewRecorder()
	FaultHandlerWithOptions(opts).ServeHTTP(rec, req)

	if rec.Body.String() != "public" {
		t.Fatalf("Unexpected body %q", rec.Body.String())
	}

	if !strings.Contains(logbuf.String(), "private") {
		t.Fatalf("Private error not logged: %q", logbuf.String())
	}
}

func TestLogInjection(t *testing.T) {

	var logbuf bytes.Buffer

	req := httptest.NewRequest("GET", "/", nil)
	req.URL.Path = "/a\n[FAULT] forged"
	req.URL.RawQuery = "token=secret"
	AssignError(req, errors.New("bad\nforged2"), 500)

	FaultHandler(log.New(&logbuf, "", 0)).ServeHTTP(httptest.NewRecorder(), req)

	out := logbuf.String()

	if strings.Count(out, "\n") != 1 {
		t.Fatalf("Log injection possible: %q", out)
	}

	if strings.Contains(out, "secret") {
		t.Fatalf("Query string logged: %q", out)
	}
}

func TestSlogLogger(t *testing.T) {

	var logbuf bytes.Buffer

	opts := &FaultHandlerOptions{
		SlogLogger: slog.New(slog.NewTextHandler(&logbuf, nil)),
	}

	req := httptest.NewRequest("GET", "/x", nil)
	AssignError(req, errors.New("oops"), 500)

	FaultHandlerWithOptions(opts).ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(logbuf.String(), "oops") {
		t.Fatalf("Nothing logged: %q", logbuf.String())
	}
}

func TestWrapperReplacesInnerBody(t *testing.T) {

	tpl := newTemplate(t, "ERR {{.Status}}")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		AssignError(r, errors.New("boom"), 404)
		http.Error(w, "inner body", 404)
	})

	rec := httptest.NewRecorder()
	TemplatedFaultHandlerWrapper(quiet, tpl, inner).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 404 || rec.Body.String() != "ERR 404" {
		t.Fatalf("Unexpected %d %q", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Length") != "" {
		t.Fatal("Stale Content-Length")
	}
}

func TestWrapperWithoutAssignedError(t *testing.T) {

	tpl := newTemplate(t, "ERR {{.Status}} {{.Error}}")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	rec := httptest.NewRecorder()
	TemplatedFaultHandlerWrapper(quiet, tpl, inner).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 404 || rec.Body.String() != "ERR 404 Not Found" {
		t.Fatalf("Unexpected %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrapperPassesThroughSuccess(t *testing.T) {

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	})

	rec := httptest.NewRecorder()
	TemplatedFaultHandlerWrapper(quiet, newTemplate(t, "x"), inner).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 201 || rec.Body.String() != "ok" {
		t.Fatalf("Unexpected %d %q", rec.Code, rec.Body.String())
	}
}

func TestStatusWriterFirstStatusWinsAndUnwrap(t *testing.T) {

	rec := httptest.NewRecorder()
	sw := NewStatusWriter(rec)

	sw.WriteHeader(201)
	sw.WriteHeader(500)

	if sw.Status != 201 {
		t.Fatalf("Unexpected status %d", sw.Status)
	}

	if sw.Unwrap() != http.ResponseWriter(rec) {
		t.Fatal("Unwrap returned the wrong writer")
	}

	err := http.NewResponseController(sw).Flush()

	if err != nil {
		t.Fatalf("Flush through ResponseController failed, %v", err)
	}
}
