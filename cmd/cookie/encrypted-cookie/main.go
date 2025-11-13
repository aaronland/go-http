package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/aaronland/go-http/v4/cookie"
)

func main() {

	name := flag.String("name", "c", "...")

	flag.Parse()

	cookie_uri, err := cookie.NewRandomEncryptedCookieURI(*name)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(cookie_uri)
}
