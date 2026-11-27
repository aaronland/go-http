package crumb

import (
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"io"
	"log/slog"
	go_http "net/http"
	"net/url"
	"strings"

	"github.com/aaronland/go-http/v4/fault"
	"github.com/aaronland/go-http/v4/rewrite"
	"github.com/aaronland/go-http/v4/sanitize"
)

// EnsureCrumbHandler wraps 'next_handler' with a middleware `http.Handler` for assigning and validating
// crumbs using the default `aaronland/go-http/v4/fault.FaultHandler` as an error handler. Any errors that
// trigger the error handler can be retrieved using `aaronland/go-http/v4/fault.RetrieveError()`.
func EnsureCrumbHandler(cr Crumb, next_handler go_http.Handler) go_http.Handler {

	logger := slog.Default()
	fault_logger := slog.NewLogLogger(logger.Handler(), slog.LevelError)

	fault_handler := fault.FaultHandler(fault_logger)
	return EnsureCrumbHandlerWithErrorHandler(cr, next_handler, fault_handler)
}

// BindFunc returns an identifier (for example a session or account ID) that crumbs for 'req' are bound to.
// An empty string means the request has no identity and the crumb is not bound to anything.
type BindFunc func(req *go_http.Request) string

// EnsureCrumbHandlerWithBinder is like `EnsureCrumbHandler` but binds crumbs to the identifier returned by 'bind'.
func EnsureCrumbHandlerWithBinder(cr Crumb, next_handler go_http.Handler, bind BindFunc) go_http.Handler {

	fault_logger := slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)
	fault_handler := fault.FaultHandler(fault_logger)
	return EnsureCrumbHandlerWithErrorHandlerAndBinder(cr, next_handler, fault_handler, bind)
}

// EnsureCrumbHandlerWithFaultWrapperAndBinder is like `EnsureCrumbHandlerWithFaultWrapper` but binds crumbs
// to the identifier returned by 'bind'.
func EnsureCrumbHandlerWithFaultWrapperAndBinder(cr Crumb, next_handler go_http.Handler, bind BindFunc) go_http.Handler {
	return ensureCrumbHandlerWithFaultWrapper(cr, next_handler, bind)
}

// bindExtra returns the variadic 'extra' arguments for Generate and Validate.
func bindExtra(bind BindFunc, req *go_http.Request) []string {

	if bind == nil {
		return nil
	}

	if id := bind(req); id != "" {
		return []string{id}
	}

	return nil
}

// EnsureCrumbHandlerWithFaultWrapper wraps 'next_handler' with a middleware `http.Handler` for assigning and validating
// crumbs. Error handling is assumed to be handled by a separate middleware handler provided by a `aaronland/go-http/v4/fault.FaultWrapper`.
// instance. Any errors that are triggered as recorded using the `aaronland/go-http/v4/fault.AssignError()` method and can
// be retrieved using `aaronland/go-http/v4/fault.RetrieveError()` method.
func EnsureCrumbHandlerWithFaultWrapper(cr Crumb, next_handler go_http.Handler) go_http.Handler {
	return ensureCrumbHandlerWithFaultWrapper(cr, next_handler, nil)
}

func ensureCrumbHandlerWithFaultWrapper(cr Crumb, next_handler go_http.Handler, bind BindFunc) go_http.Handler {

	fn := func(rsp go_http.ResponseWriter, req *go_http.Request) {

		switch req.Method {

		case go_http.MethodPost, go_http.MethodPut, go_http.MethodPatch, go_http.MethodDelete:

			crumb_var, crumb_err := crumbFromRequest(req)

			if crumb_err != nil {
				err := Error(UnsanitizedCrumb, crumb_err)
				fault.AssignError(req, err, go_http.StatusBadRequest)
				rsp.WriteHeader(go_http.StatusBadRequest)
				return
			}

			if crumb_var == "" {
				err := Error(MissingCrumb, fmt.Errorf("Missing crumb"))
				fault.AssignError(req, err, go_http.StatusBadRequest)
				rsp.WriteHeader(go_http.StatusBadRequest)
				return
			}

			ok, err := cr.Validate(req, crumb_var, bindExtra(bind, req)...)

			if err != nil {
				err := Error(InvalidCrumb, err)
				fault.AssignError(req, err, go_http.StatusBadRequest)
				rsp.WriteHeader(go_http.StatusBadRequest)
				return
			}

			if !ok {
				err := Error(ExpiredCrumb, fmt.Errorf("Expired"))
				fault.AssignError(req, err, go_http.StatusForbidden)
				rsp.WriteHeader(go_http.StatusForbidden)
				return
			}

		default:
			// pass
		}

		crumb_var, err := cr.Generate(req, bindExtra(bind, req)...)

		if err != nil {
			err := Error(GenerateCrumb, fmt.Errorf("Expired"))
			fault.AssignError(req, err, go_http.StatusInternalServerError)
			rsp.WriteHeader(go_http.StatusInternalServerError)
			return
		}

		rewrite_func := NewCrumbRewriteFunc(crumb_var)
		rewrite_handler := rewrite.RewriteHTMLHandler(next_handler, rewrite_func)

		rewrite_handler.ServeHTTP(rsp, req)
	}

	h := go_http.HandlerFunc(fn)
	return h
}

// EnsureCrumbHandlerWithErrorHandler wraps 'next_handler' with a middleware a middleware `http.Handler` for
// assigning and validating crumbs using a custom error handler. Any errors that trigger the error handler can
// be retrieved using `aaronland/go-http/v4/fault.RetrieveError()`.
func EnsureCrumbHandlerWithErrorHandler(cr Crumb, next_handler go_http.Handler, error_handler go_http.Handler) go_http.Handler {
	return EnsureCrumbHandlerWithErrorHandlerAndBinder(cr, next_handler, error_handler, nil)
}

// EnsureCrumbHandlerWithErrorHandlerAndBinder is like `EnsureCrumbHandlerWithErrorHandler` but binds crumbs to
// the identifier returned by 'bind' (which may be nil).
func EnsureCrumbHandlerWithErrorHandlerAndBinder(cr Crumb, next_handler go_http.Handler, error_handler go_http.Handler, bind BindFunc) go_http.Handler {

	fn := func(rsp go_http.ResponseWriter, req *go_http.Request) {

		switch req.Method {

		case go_http.MethodPost, go_http.MethodPut, go_http.MethodPatch, go_http.MethodDelete:

			crumb_var, crumb_err := crumbFromRequest(req)

			if crumb_err != nil {
				fault.AssignError(req, Error(UnsanitizedCrumb, crumb_err), go_http.StatusBadRequest)
				error_handler.ServeHTTP(rsp, req)
				return
			}

			if crumb_var == "" {
				fault.AssignError(req, Error(MissingCrumb, fmt.Errorf("Missing crumb")), go_http.StatusBadRequest)
				error_handler.ServeHTTP(rsp, req)
				return
			}

			ok, err := cr.Validate(req, crumb_var, bindExtra(bind, req)...)

			if err != nil {
				fault.AssignError(req, Error(InvalidCrumb, err), go_http.StatusInternalServerError)
				error_handler.ServeHTTP(rsp, req)
				return
			}

			if !ok {
				fault.AssignError(req, Error(ExpiredCrumb, fmt.Errorf("Expired")), go_http.StatusForbidden)
				error_handler.ServeHTTP(rsp, req)
				return
			}

		default:
			// pass
		}

		crumb_var, err := cr.Generate(req, bindExtra(bind, req)...)

		if err != nil {
			fault.AssignError(req, Error(GenerateCrumb, err), go_http.StatusInternalServerError)
			error_handler.ServeHTTP(rsp, req)
			return
		}

		rewrite_func := NewCrumbRewriteFunc(crumb_var)
		rewrite_handler := rewrite.RewriteHTMLHandler(next_handler, rewrite_func)

		rewrite_handler.ServeHTTP(rsp, req)
	}

	h := go_http.HandlerFunc(fn)
	return h
}

// CrumbHeader is the HTTP header that may be used to pass a crumb with PUT, PATCH and DELETE requests.
const CrumbHeader = "X-Crumb"

// crumbFromRequest returns the crumb for 'req'. POST requests read the "crumb" form value; all other
// unsafe methods read the `X-Crumb` header, falling back to the "crumb" query parameter.
func crumbFromRequest(req *go_http.Request) (string, error) {

	if req.Method == go_http.MethodPost {
		return sanitize.PostString(req, "crumb")
	}

	if v := req.Header.Get(CrumbHeader); v != "" {
		return v, nil
	}

	return sanitize.GetString(req, "crumb")
}

// isSameOriginPostForm returns true if the "form" node 'n' submits via POST to a relative URL.
func isSameOriginPostForm(n *html.Node) bool {

	method := ""
	action := ""

	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "method":
			method = strings.ToLower(strings.TrimSpace(a.Val))
		case "action":
			action = strings.TrimSpace(a.Val)
		}
	}

	if method != "post" {
		return false
	}

	u, err := url.Parse(action)

	if err != nil {
		return false
	}

	return u.Host == "" && u.Scheme == ""
}

// NewCrumbRewriteFunc returns a `aaronland/go-http-rewrite.RewriteHTMLFunc` used to
// append crumb data to HTML output.
func NewCrumbRewriteFunc(crumb_var string) rewrite.RewriteHTMLFunc {

	var rewrite_func rewrite.RewriteHTMLFunc

	rewrite_func = func(n *html.Node, w io.Writer) {

		if n.Type == html.ElementNode && n.Data == "body" {

			crumb_ns := ""
			crumb_key := "data-crumb"
			crumb_value := crumb_var

			crumb_attr := html.Attribute{crumb_ns, crumb_key, crumb_value}
			n.Attr = append(n.Attr, crumb_attr)
		}

		if n.Type == html.ElementNode && n.Data == "form" && isSameOriginPostForm(n) {

			ns := ""

			attrs := []html.Attribute{
				html.Attribute{ns, "type", "hidden"},
				html.Attribute{ns, "id", "crumb"},
				html.Attribute{ns, "name", "crumb"},
				html.Attribute{ns, "value", crumb_var},
			}

			i := &html.Node{
				Type:      html.ElementNode,
				DataAtom:  atom.Input,
				Data:      "input",
				Namespace: ns,
				Attr:      attrs,
			}

			n.AppendChild(i)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rewrite_func(c, w)
		}
	}

	return rewrite_func
}
