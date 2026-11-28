# go-http/v4/cookie

Go package for working with HTTP cookies.

The `Cookie` interface reads, writes and deletes cookies (as strings or as `memguard.LockedBuffer` values). Implementations register a URI scheme and are created with `NewCookie`. The `Config` type (created by `NewConfig`) derives cookie attributes (name, domain, path, `Secure`, `SameSite` and TTL) from a URI.

## Schemes

### encrypted://?name={NAME}&secret={SECRET}&salt={SALT}

Cookies whose values are encrypted and authenticated using [NaCl secretbox](https://godoc.org/golang.org/x/crypto/nacl/secretbox). The `secret` must be exactly 32 bytes. `NewRandomEncryptedCookieURI` will generate a URI with random values.

## Example

_Error handling omitted for the sake of brevity._

```
package main

import (
	"fmt"
	"github.com/aaronland/go-http/v4/cookie"	
)

func main() {

	name := "c"
	secret := "0123456789abcdef0123456789abcdef" // must be exactly 32 bytes
	salt := "s4lty" // required but unused; the secret is used directly as the key
	
	cookie_uri := fmt.Sprintf("encrypted://?name=%s&secret=%s&salt=%s", name, secret, salt)
	ck, _ := cookie.NewCookie(ctx, cookie_uri)
}
```

## See also

* https://godoc.org/golang.org/x/crypto/nacl/secretbox
* https://github.com/awnumar/memguard