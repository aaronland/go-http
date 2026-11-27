# go-http/v4/crumb

Go package for creating and validating (HTTP) crumbs.

## Example

_The following are abbreviated code examples. Error handling has been omitted for the sake of brevity._

### Simple (Encypted crumbs)

```
import (
       "context"
       
       "github.com/aaronland/go-http/v4/crumb"
)

func main() {
	cr_uri := "encrypted://?extra=f3gKgLVX&key=&secret=oK5OFCjBsvAOrfJPnzAJqnkphkuDmyf9&separator=%3A&ttl=3600"
	cr, _ := crumb.NewCrumb(ctx, uri)
}

```

### HTTP (Simple)

Use the `crumb.EnsureCrumbHandler` middleware handler to automatically generate a new crumb string for all requests and append it to any HTML output as a `html/body@data-crumb` attribute value.

For `POST`, `PUT`, `PATCH` and `DELETE` requests the (middleware) handler intercepts the current handler and validates the crumb before continuing. `POST` requests are expected to have a `crumb` form value. Other methods may send an `X-Crumb` header (or, failing that, a `crumb` query parameter). Crumb inputs are only added to `<form method="post">` elements with relative `action` URLs.

By default crumbs are _not_ bound to a user or session: any page that generates a crumb yields one that is valid for any client with the same User-Agent and key. Use the "Binding crumbs to a session" approach below for authenticated applications.

```
import (
       "context"
       "net/http"

       "github.com/aaronland/go-http/v4/crumb"
)

func MyHandler() http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {
		rsp.Write([]byte("Hello world"))
	}

	return http.HandlerFunc(fn)
}

func main() {

	ctx := context.Background()
	
	uri, _ := crumb.NewRandomEncryptedCrumbURI(ctx, 3600)
	cr, _ := crumb.NewCrumb(ctx, uri)

	mux := http.NewServeMux()
	
	my_handler, _ := MyHandler()
	my_handler = crumb.EnsureCrumbHandler(cr, my_handler)

	mux.Handle("/", my_handler)
}
```

### HTTP (Binding crumbs to a session)

The `crumb.EnsureCrumbHandlerWithBinder` (and `EnsureCrumbHandlerWithErrorHandlerAndBinder`, `EnsureCrumbHandlerWithFaultWrapperAndBinder`) middleware accept a `crumb.BindFunc`: a `func(*http.Request) string` returning an identifier (for example an account or session ID) that is mixed in when crumbs are generated and validated. A crumb issued for one identifier will not validate for another. Returning an empty string leaves the crumb unbound (for example, for pre-signin pages). The existing handlers are unchanged and never bind.

The binder is called on every request, so it must be able to find the identity itself; this example reads the account directly from the authenticator.

```
bind := func(req *http.Request) string {

	acct, err := authenticator.GetAccountForRequest(req)

	if err != nil {
		return ""
	}

	return acct.Id()
}

my_handler = crumb.EnsureCrumbHandlerWithBinder(cr, my_handler, bind)
```

Note that some authenticators (for example, shared secret) return accounts with an empty `Id()`, in which case crumbs are not bound. Return a different identifier (such as a session cookie value) in that case.

### HTTP (Doing it yourself)

```
import (
       "context"
       "net/http"

       "github.com/aaronland/go-http/v4/crumb"
)

func CrumbHandler() (http.Handler, error) {

	ctx := context.Background()
	
	uri, _ := crumb.NewRandomEncryptedCrumbURI(ctx, 3600)
	cr, _ := crumb.NewCrumb(ctx, uri)

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		if req.URL.Method == "GET" {
			// optionally pass a session identifier: cr.Generate(req, session_id)
			cr_hash, _ := cr.Generate(req)
			// pass cr_hash to template
		} else {

			// read cr_hash from POST form here
			ok, _ := cr.Validate(req, cr_hash)	
		}
	}

	h := http.HandlerFunc(fn)
	return h, nil
}
```

## Schemes

### encrypted:///?secret={SECRET}&extra={EXTRA}&ttl={TTL}&separator={SEPARATOR}

For example:

```
encrypted:///?secret={SECRET}&extra={EXTRA}&ttl={TTL}&separator={SEPARATOR}
```

| Parameter | Description | Required |
| --- | --- | --- |
| secret | An AES secret for encrypting the crumb. Must be 16, 24 or 32 bytes long. Crumbs are encrypted with AES-GCM. | yes |
| extra | A string to include when generating crumb base | yes |
| separator | A string to separate crumb parts with | yes |
| ttl | Time to live (in seconds) | yes |
| key | A string to prepend crumb base with. Default is to use the path of the current HTTP request | no |