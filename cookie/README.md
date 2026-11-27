# go-http/v4/cookie

Go package for working with HTTP cookies.

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