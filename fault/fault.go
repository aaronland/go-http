// Package fault provides HTTP handlers and middleware for reporting errors in web applications,
// optionally rendering them with a custom HTML template.
package fault

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"reflect"
)

// ErrorKey is the name of the legacy string key for assigning `error` values to a `context.Context` instance.
//
// Deprecated: use `AssignError` and `RetrieveError`. The value is still assigned for backwards compatibility.
const ErrorKey string = "github.com/sfomuseum/go-http-fault#error"

// StatusKey is the name of the legacy string key for assigning status code values to a `context.Context` instance.
//
// Deprecated: use `AssignError` and `RetrieveError`. The value is still assigned for backwards compatibility.
const StatusKey string = "github.com/sfomuseum/go-http-fault#status"

type contextKey int

const (
	errorContextKey contextKey = iota
	statusContextKey
)

// FaultHandlerOptions is a struct containing configuration options for the `FaultHandlerWithOptions` method.
type FaultHandlerOptions struct {
	// Logger is a custom `log.Logger` instance used for logging and feedback. If nil (and `SlogLogger` is
	// also nil) `log.Default()` is used.
	Logger *log.Logger
	// SlogLogger is an optional `log/slog.Logger`. If set it is used instead of `Logger`.
	SlogLogger *slog.Logger
	// Template is an optional `html/template.Template` to use for reporting errors.
	Template *template.Template
	// VarsFunc is a `FaultHandlerVarsFunc` used to derive variables passed to templates. If nil and `Template`
	// is non-nil the default `FaultHandlerVars` are used. The function is called once per request and
	// should return a new value each time; values returned by pointer are copied before being modified.
	VarsFunc FaultHandlerVarsFunc
	// GenericPublicErrors, if true, replaces the public error of any error which does not implement `FaultError`
	// with `http.StatusText(status)` so that internal error text is never passed to templates. The full error
	// is still logged.
	GenericPublicErrors bool
}

// FaultHandlerVars are the minimal required variables that will be passed to a fault handler template. If you need to pass additional fields
// then you will need to create your handler using the `FaultHandlerWithOptions` method specifying a custom `FaultHandlerVarsFunc` property.
type FaultHandlerVars struct {
	// Status is the HTTP status code of the error being reported.
	Status int
	// Error is the public error being reported.
	Error error
}

// FaultHandlerVarsSetter is an optional interface that custom template variables may implement, as an
// alternative to having exported `Status` and `Error` fields, to receive the status and public error.
type FaultHandlerVarsSetter interface {
	SetFaultVars(status int, public_err error)
}

// FaultHandlerVarsFunc is a custom function that returns a pointer to a struct that conforms to the required fields of the `FaultHandlerVars` struct type
// or that implements the `FaultHandlerVarsSetter` interface.
type FaultHandlerVarsFunc func() any

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// varsStruct returns the (addressable) struct that 'vars' points to if it has settable `Status` (int) and `Error` (error) fields.
func varsStruct(vars any) (reflect.Value, error) {

	if vars == nil {
		return reflect.Value{}, errors.New("template vars are nil")
	}

	v := reflect.ValueOf(vars)

	if v.Kind() != reflect.Pointer {
		return reflect.Value{}, errors.New("template vars must be a pointer")
	}

	if v.IsNil() {
		return reflect.Value{}, errors.New("template vars are a nil pointer")
	}

	dv := v.Elem()

	if dv.Kind() != reflect.Struct {
		return reflect.Value{}, errors.New("template vars must be a pointer to a struct")
	}

	s := dv.FieldByName("Status")

	if !s.IsValid() || !s.CanSet() || s.Kind() != reflect.Int {
		return reflect.Value{}, errors.New("template vars have no settable int field 'Status'")
	}

	e := dv.FieldByName("Error")

	if !e.IsValid() || !e.CanSet() || e.Type() != errorType {
		return reflect.Value{}, errors.New("template vars have no settable error field 'Error'")
	}

	return dv, nil
}

// ImplementsFaultHandlerVars returns a boolean value indicating whether 'vars' conforms to the required fields of the `FaultHandlerVars` struct type
// or implements the `FaultHandlerVarsSetter` interface.
func ImplementsFaultHandlerVars(vars any) bool {

	switch vars.(type) {
	case FaultHandlerVars, *FaultHandlerVars, FaultHandlerVarsSetter:
		return true
	default:
		// carry on
	}

	_, err := varsStruct(vars)
	return err == nil
}

// populateVars returns a copy of 'vars' with its status and error set. Pointers are copied so that a
// value shared by every call to a `FaultHandlerVarsFunc` is never modified concurrently.
func populateVars(vars any, status int, public_err error) (any, error) {

	switch v := vars.(type) {
	case FaultHandlerVars:
		v.Status = status
		v.Error = public_err
		return v, nil
	case *FaultHandlerVars:
		if v == nil {
			return nil, errors.New("template vars are a nil pointer")
		}
		c := *v
		c.Status = status
		c.Error = public_err
		return &c, nil
	case FaultHandlerVarsSetter:
		v.SetFaultVars(status, public_err)
		return v, nil
	}

	dv, err := varsStruct(vars)

	if err != nil {
		return nil, err
	}

	nv := reflect.New(dv.Type())
	nv.Elem().Set(dv)

	nv.Elem().FieldByName("Status").SetInt(int64(status))
	nv.Elem().FieldByName("Error").Set(reflect.ValueOf(&public_err).Elem())

	return nv.Interface(), nil
}

func defaultFaultHandlerVars() any {
	return &FaultHandlerVars{
		Status: 0,
		Error:  fmt.Errorf("Undefined error"),
	}
}

// AssignError assigns 'err' and 'status' to the context of 'req' and updates 'req' in place
// (that is, '*req' is replaced by a shallow copy carrying the new context). The values are also assigned
// to the legacy `ErrorKey` and `StatusKey` string keys.
func AssignError(req *http.Request, err error, status int) {

	ctx := req.Context()
	ctx = context.WithValue(ctx, errorContextKey, err)
	ctx = context.WithValue(ctx, statusContextKey, status)

	// Legacy keys, retained for callers which read them directly.
	ctx = context.WithValue(ctx, ErrorKey, err)
	ctx = context.WithValue(ctx, StatusKey, status)

	new_req := req.WithContext(ctx)
	*req = *new_req
}

// RetrieveError returns the status code and error assigned to 'req' with `AssignError`.
// If either value is missing, or of the wrong type, a status of `http.StatusInternalServerError` and
// a generic error are used instead.
func RetrieveError(req *http.Request) (int, error) {

	ctx := req.Context()

	err_v := ctx.Value(errorContextKey)

	if err_v == nil {
		err_v = ctx.Value(ErrorKey)
	}

	status_v := ctx.Value(statusContextKey)

	if status_v == nil {
		status_v = ctx.Value(StatusKey)
	}

	status := http.StatusInternalServerError
	err, ok := err_v.(error)

	if !ok || err == nil {
		err = errors.New("FaultHandler triggered without an error context.")
	}

	s, ok := status_v.(int)

	if ok {
		status = s
	}

	return status, err
}

// FaultHandler returns a `http.Handler` instance for handling errors in a web application.
func FaultHandler(l *log.Logger) http.Handler {

	opts := &FaultHandlerOptions{
		Logger:   l,
		Template: nil,
		VarsFunc: nil, // unnecessary because no template
	}

	return FaultHandlerWithOptions(opts)
}

// TemplatedFaultHandler returns a `http.Handler` instance for handling errors in a web application
// with a custom HTML template.
func TemplatedFaultHandler(l *log.Logger, t *template.Template) http.Handler {

	opts := &FaultHandlerOptions{
		Logger:   l,
		Template: t,
		VarsFunc: defaultFaultHandlerVars,
	}

	return FaultHandlerWithOptions(opts)
}

// logFault records a fault, quoting request-controlled values so they cannot forge log lines. Only the
// URL path is logged, not the query string, which may contain secrets.
func logFault(opts *FaultHandlerOptions, req *http.Request, msg string) {

	path := ""

	if req.URL != nil {
		path = req.URL.Path
	}

	if opts.SlogLogger != nil {
		opts.SlogLogger.Error("fault", "remote_addr", req.RemoteAddr, "method", req.Method, "path", path, "proto", req.Proto, "error", msg)
		return
	}

	opts.Logger.Printf("[FAULT] addr=%q method=%q path=%q proto=%q error=%q", req.RemoteAddr, req.Method, path, req.Proto, msg)
}

// FaultHandlerWithOptions returns a `http.Handler` for handling errors in a web application. It will retrieve
// any "public" and "private" errors that have been recorded and log the private error to 'opts.Logger'. If 'opts.Template' is
// defined it will be executed and passed the "public" error and status as template variables. The response
// has the status code assigned with `AssignError` (or 500 if that value is outside the range 100-999 accepted by `net/http`).
func FaultHandlerWithOptions(opts *FaultHandlerOptions) http.Handler {

	// Work on a copy so the caller's options are never modified.
	o := FaultHandlerOptions{}

	if opts != nil {
		o = *opts
	}

	if o.Logger == nil {
		o.Logger = log.Default()
	}

	if o.Template != nil && o.VarsFunc == nil {
		o.VarsFunc = defaultFaultHandlerVars
	}

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		status, err := RetrieveError(req)

		// net/http panics on codes outside 100-999.
		if status < 100 || status > 999 {
			status = http.StatusInternalServerError
		}

		var fault_err FaultError

		public_err := err
		private_err := err

		if errors.As(err, &fault_err) {
			public_err = fault_err.Public()
			private_err = fault_err.Private()
		} else if o.GenericPublicErrors {
			public_err = errors.New(http.StatusText(status))
		}

		if public_err == nil {
			public_err = errors.New(http.StatusText(status))
		}

		if private_err == nil {
			private_err = public_err
		}

		logFault(&o, req, private_err.Error())

		if o.Template != nil {

			body, err := renderTemplate(&o, status, public_err)

			if err == nil {
				h := rsp.Header()
				h.Set("Content-Type", "text/html; charset=utf-8")
				h.Set("X-Content-Type-Options", "nosniff")
				h.Del("Content-Length")

				rsp.WriteHeader(status)
				rsp.Write(body)
				return
			}

			logFault(&o, req, fmt.Sprintf("Failed to render template for fault handler, %v", err))
		}

		err_msg := fmt.Sprintf("There was a problem completing your request (%d)", status)
		http.Error(rsp, err_msg, status)
	}

	return http.HandlerFunc(fn)
}

// renderTemplate executes the template in memory so that nothing is sent to the client if it fails.
func renderTemplate(opts *FaultHandlerOptions, status int, public_err error) (body []byte, err error) {

	defer func() {
		r := recover()

		if r != nil {
			err = fmt.Errorf("panic rendering template: %v", r)
		}
	}()

	vars, err := populateVars(opts.VarsFunc(), status, public_err)

	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	err = opts.Template.Execute(&buf, vars)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
