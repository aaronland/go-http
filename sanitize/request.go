package sanitize

import (
	"net/http"
)

// RequestString returns a sanitised string for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestString(req *http.Request, param string) (string, error) {

	switch req.Method {

	case "POST":
		return PostString(req, param)
	default:
		return GetString(req, param)
	}

}

// RequestInt returns a sanitised int for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestInt(req *http.Request, param string) (int, error) {

	switch req.Method {

	case "POST":
		return PostInt(req, param)
	default:
		return GetInt(req, param)
	}

}

// RequestInt64 returns a sanitised int64 for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestInt64(req *http.Request, param string) (int64, error) {

	switch req.Method {

	case "POST":
		return PostInt64(req, param)
	default:
		return GetInt64(req, param)
	}

}

// RequestFloat64 returns a sanitised float64 for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestFloat64(req *http.Request, param string) (float64, error) {

	switch req.Method {

	case "POST":
		return PostFloat64(req, param)
	default:
		return GetFloat64(req, param)
	}

}

// RequestBool returns a sanitised bool for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestBool(req *http.Request, param string) (bool, error) {

	switch req.Method {

	case "POST":
		return PostBool(req, param)
	default:
		return GetBool(req, param)
	}

}
