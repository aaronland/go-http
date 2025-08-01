# go-http/v3/cookie

Go package for working with HTTP cookies.

## Example

_Error handling omitted for the sake of brevity._

```
package main

import (
	"fmt"
	"github.com/aaronland/go-http/v3/cookie"	
)

func main() {

	name := "c"
	secret := "s33kret"
	salt := "s4lty"
	
	cookie_uri := fmt.Sprintf("encrypted://?name=%s&secret=%s&salt=%s", name, secret, salt)
	ck, _ := cookie.NewCookie(ctx, cookie_uri)
}
```

## See also

* https://github.com/aaronland/go-secretbox
* https://godoc.org/golang.org/x/crypto/nacl/secretbox
* https://github.com/awnumar/memguard