// Command jwt-secret prints a random string suitable for use as a JWT signing secret.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" +
	"-_"

var seededRand *rand.Rand = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()*2)))

// StringWithCharset returns a random string of 'length' characters drawn from 'charset'.
func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.IntN(len(charset))]
	}
	return string(b)
}

// String returns a random string of 'length' characters drawn from the default character set.
func String(length int) string {
	return StringWithCharset(length, charset)
}

func main() {

	var length int

	flag.IntVar(&length, "length", 256, "...")
	flag.Parse()

	fmt.Printf("%s", String(length))
}
