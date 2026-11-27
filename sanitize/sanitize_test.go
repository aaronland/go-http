package sanitize

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func getReq(query string) *http.Request {
	return httptest.NewRequest("GET", "/?"+query, nil)
}

func postReq(form url.Values) *http.Request {
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestGetTypedValues(t *testing.T) {

	req := getReq("s=hello&i=42&i64=9007199254740993&f=1.5&b=true&empty=&bad=abc")

	if v, err := GetString(req, "s"); err != nil || v != "hello" {
		t.Errorf("GetString: %q %v", v, err)
	}

	if v, err := GetInt(req, "i"); err != nil || v != 42 {
		t.Errorf("GetInt: %d %v", v, err)
	}

	if v, err := GetInt64(req, "i64"); err != nil || v != 9007199254740993 {
		t.Errorf("GetInt64: %d %v", v, err)
	}

	if v, err := GetFloat64(req, "f"); err != nil || v != 1.5 {
		t.Errorf("GetFloat64: %v %v", v, err)
	}

	if v, err := GetBool(req, "b"); err != nil || !v {
		t.Errorf("GetBool: %v %v", v, err)
	}

	// Empty and absent parameters are zero values, not errors.
	for _, p := range []string{"empty", "absent"} {

		if v, err := GetInt(req, p); err != nil || v != 0 {
			t.Errorf("GetInt(%s): %d %v", p, v, err)
		}

		if v, err := GetBool(req, p); err != nil || v {
			t.Errorf("GetBool(%s): %v %v", p, v, err)
		}
	}

	if _, err := GetInt(req, "bad"); err == nil {
		t.Error("GetInt should fail for a non-number")
	}

	if _, err := GetFloat64(req, "bad"); err == nil {
		t.Error("GetFloat64 should fail for a non-number")
	}

	if _, err := GetBool(req, "bad"); err == nil {
		t.Error("GetBool should fail for a non-bool")
	}
}

func TestPostTypedValues(t *testing.T) {

	form := url.Values{"s": {"hello"}, "i": {"7"}, "f": {"2.5"}, "b": {"false"}}

	if v, err := PostString(postReq(form), "s"); err != nil || v != "hello" {
		t.Errorf("PostString: %q %v", v, err)
	}

	if v, err := PostInt(postReq(form), "i"); err != nil || v != 7 {
		t.Errorf("PostInt: %d %v", v, err)
	}

	if v, err := PostFloat64(postReq(form), "f"); err != nil || v != 2.5 {
		t.Errorf("PostFloat64: %v %v", v, err)
	}

	if v, err := PostBool(postReq(form), "b"); err != nil || v {
		t.Errorf("PostBool: %v %v", v, err)
	}
}

func TestRequestDispatchesOnMethod(t *testing.T) {

	// A POST must read the body and ignore the query string, and vice versa for GET.
	post := httptest.NewRequest("POST", "/?name=fromquery", strings.NewReader("name=frombody"))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if v, _ := RequestString(post, "name"); v != "frombody" {
		t.Errorf("POST: got %q", v)
	}

	if v, _ := RequestString(getReq("name=fromquery"), "name"); v != "fromquery" {
		t.Errorf("GET: got %q", v)
	}
}

func TestMultiValues(t *testing.T) {

	req := getReq("n=1&n=2&n=3&s=a&s=b")

	ints, err := GetIntMulti(req, "n")

	if err != nil || len(ints) != 3 || ints[0] != 1 || ints[2] != 3 {
		t.Errorf("GetIntMulti: %v %v", ints, err)
	}

	strs, err := GetStringMulti(req, "s")

	if err != nil || len(strs) != 2 {
		t.Errorf("GetStringMulti: %v %v", strs, err)
	}

	if _, err := GetIntMulti(getReq("n=1&n=x"), "n"); err == nil {
		t.Error("GetIntMulti should fail if any value is not a number")
	}
}

func TestHeaderValues(t *testing.T) {

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Count", "12")
	req.Header.Set("X-Flag", "true")

	if v, err := HeaderInt64(req, "X-Count"); err != nil || v != 12 {
		t.Errorf("HeaderInt64: %d %v", v, err)
	}

	if v, err := HeaderBool(req, "X-Flag"); err != nil || !v {
		t.Errorf("HeaderBool: %v %v", v, err)
	}

	if v, err := HeaderBool(req, "X-Missing"); err != nil || v {
		t.Errorf("HeaderBool missing: %v %v", v, err)
	}
}

// The sanitizer strips control characters; it does not escape HTML, so
// callers must still escape values on output. This pins both behaviours.
func TestHostileInput(t *testing.T) {

	controls := []string{"line1\r\nSet-Cookie: pwned=1", "null\x00byte", "bell\x07", "del\x7f", "bom\ufeffx", "rtl\u202ex"}

	for _, p := range controls {
		t.Run(fmt.Sprintf("%q", p), func(t *testing.T) {

			v, err := GetString(getReq("x="+url.QueryEscape(p)), "x")

			if err != nil {
				return
			}

			for _, r := range v {
				if r == '\x00' || r == '\x07' || r == '\x7f' || r == '\ufeff' || r == '\u202e' {
					t.Errorf("control character %U survived in %q", r, v)
				}
			}
		})
	}

	v, err := GetString(getReq("x="+url.QueryEscape("<b>")), "x")

	if err != nil || v != "<b>" {
		t.Errorf("markup is expected to pass through unchanged, got %q %v", v, err)
	}
}

func FuzzGetString(f *testing.F) {

	for _, s := range []string{"", "a", "\x00", "%00", "é", "\u202e", strings.Repeat("A", 4096)} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {

		req := httptest.NewRequest("GET", "/", nil)
		req.URL.RawQuery = url.Values{"x": {s}}.Encode()

		v, err := GetString(req, "x")

		if err != nil {
			return
		}

		for _, r := range v {
			if r < 0x09 || (r > 0x0d && r < 0x20) || r == 0x7f {
				t.Fatalf("control character %U survived: %q -> %q", r, s, v)
			}
		}
	})
}
