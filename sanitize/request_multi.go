package sanitize

import (
	"net/http"
)

// RequestStringMulti returns sanitised strings for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestStringMulti(req *http.Request, param string) ([]string, error) {

	switch req.Method {

	case "POST":
		return PostStringMulti(req, param)
	default:
		return GetStringMulti(req, param)
	}

}

// RequestIntMulti returns sanitised ints for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestIntMulti(req *http.Request, param string) ([]int, error) {

	switch req.Method {

	case "POST":
		return PostIntMulti(req, param)
	default:
		return GetIntMulti(req, param)
	}

}

// RequestInt64Multi returns sanitised int64 values for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestInt64Multi(req *http.Request, param string) ([]int64, error) {

	switch req.Method {

	case "POST":
		return PostInt64Multi(req, param)
	default:
		return GetInt64Multi(req, param)
	}

}

// RequestFloat64Multi returns sanitised float64 values for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestFloat64Multi(req *http.Request, param string) ([]float64, error) {

	switch req.Method {

	case "POST":
		return PostFloat64Multi(req, param)
	default:
		return GetFloat64Multi(req, param)
	}

}

// RequestBoolMulti returns sanitised bool values for 'param' from the POST form body if the request method is POST,
// otherwise from the URL query parameters.
func RequestBoolMulti(req *http.Request, param string) ([]bool, error) {

	switch req.Method {

	case "POST":
		return PostBoolMulti(req, param)
	default:
		return GetBoolMulti(req, param)
	}

}
