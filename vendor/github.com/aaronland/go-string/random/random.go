// package random provides methods for generating random strings.
package random

import (
	cryptorand "crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const min_length int = 32

// max_length is the upper bound for the `Length` and `Chars` options.
const max_length int = 1 << 20

// rand_buffer_size is the number of bytes read from `crypto/rand` at a time.
const rand_buffer_size int = 1024

// The package level character pools are only written in `init` and are read-only afterwards.
var runes []rune

var ascii_runes []rune

var alpha_numeric_runes []rune

var emoji_runes []rune

// randSource reads bytes from `crypto/rand` in bulk and hands out unbiased
// random integers. It is not safe for concurrent use; `String` creates one per call.
type randSource struct {
	buf []byte
	pos int
}

func newRandSource() *randSource {
	return &randSource{
		buf: make([]byte, rand_buffer_size),
		pos: rand_buffer_size,
	}
}

func (s *randSource) uint32() (uint32, error) {

	if s.pos+4 > len(s.buf) {

		_, err := cryptorand.Read(s.buf)

		if err != nil {
			return 0, err
		}

		s.pos = 0
	}

	b := s.buf[s.pos : s.pos+4]
	s.pos += 4

	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// intn returns a uniformly distributed integer in [0, n) using rejection sampling to avoid modulo bias.
func (s *randSource) intn(n int) (int, error) {

	if n <= 0 || uint64(n) > 1<<32 {
		return 0, fmt.Errorf("invalid max random value: %d", n)
	}

	bound := uint64(n)
	limit := (1 << 32 / bound) * bound

	for {

		v, err := s.uint32()

		if err != nil {
			return 0, err
		}

		if uint64(v) < limit {
			return int(uint64(v) % bound), nil
		}
	}
}

func init() {

	runes = make([]rune, 0)
	ascii_runes = make([]rune, 0)
	alpha_numeric_runes = make([]rune, 0)

	codepoints := [][]int{
		[]int{1, 255},         // ascii
		[]int{127744, 128317}, // emoji
	}

	alpha_numeric := [][]int{
		[]int{48, 57},  // (0-9)
		[]int{65, 90},  // (A-Z)
		[]int{97, 122}, // (a-z)
	}

	for _, r := range codepoints {

		first := r[0]
		last := r[1]

		for i := first; i <= last; i++ {

			r := rune(i)

			if unicode.IsControl(r) {
				continue
			}

			// Format characters (for example the soft hyphen, U+00AD) are invisible
			if unicode.Is(unicode.Cf, r) {
				continue
			}

			if unicode.IsSpace(r) {
				continue
			}

			if unicode.IsMark(r) {
				continue
			}

			runes = append(runes, r)

			if r <= 127 {
				ascii_runes = append(ascii_runes, r)
			}

			if i >= 127744 {
				emoji_runes = append(emoji_runes, r)
			}

			for _, bookends := range alpha_numeric {

				if i >= bookends[0] && i <= bookends[1] {
					alpha_numeric_runes = append(alpha_numeric_runes, r)
					break
				}
			}
		}
	}

}

// Options is a struct containing configuration options for the `String` method.
type Options struct {
	// The length in bytes of the final string. Note that most of the default
	// alphabet is made up of multi-byte characters so a string of 'Length' bytes
	// contains far fewer characters (and far less entropy) than an ASCII or
	// alphanumeric string of the same length. Use `ASCII` or `AlphaNumeric` for secrets.
	Length int
	// The number of characters in the final string. If greater than 0 it takes precedence over `Length`.
	Chars int
	// A boolean flag indicating that only ASCII characters be included in the final string.
	ASCII bool
	// A boolean flag indicating that only alphanumeric (a-zA-Z0-9) characters be included in the final string.
	AlphaNumeric bool
	// A boolean flag indicating that only emoji be included in the final string. It can not be combined
	// with `ASCII` or `AlphaNumeric`. Every emoji is 4 bytes long so `Length` must be at least 4 and is
	// rounded down to a multiple of 4.
	Emoji bool
	// A boolean flag indicating the random string should be returned as a base32 encoded string.
	Base32 bool
}

// DefaultOptions returns an `Options` instance with no limits or restrictions save a minimum length of 32 bytes.
func DefaultOptions() *Options {

	opts := Options{
		Length:       min_length,
		Chars:        0,
		ASCII:        false,
		AlphaNumeric: false,
		Base32:       false,
	}

	return &opts
}

// Bytes returns 'n' bytes read from `crypto/rand`. Unlike `String` the entropy is exactly 8 bits per byte.
// 'n' must be greater than 0 and no larger than 1048576. It is safe for concurrent use.
func Bytes(n int) ([]byte, error) {

	if n <= 0 || n > max_length {
		return nil, fmt.Errorf("invalid number of bytes: %d", n)
	}

	b := make([]byte, n)

	_, err := cryptorand.Read(b)

	if err != nil {
		return nil, err
	}

	return b, nil
}

// Token returns a URL-safe (unpadded base64) string encoding at least 'bits' bits of entropy from
// `crypto/rand`. Prefer it over `String` for secrets, API keys and session identifiers. 'bits' must
// be greater than 0 and no larger than 8388608. It is safe for concurrent use.
func Token(bits int) (string, error) {

	if bits <= 0 || bits > max_length*8 {
		return "", fmt.Errorf("invalid number of bits: %d", bits)
	}

	b, err := Bytes((bits + 7) / 8)

	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// String returns a randomly-generated string configured by 'opts'. If 'opts'
// is nil the result of DefaultOptions is used. It returns an error if neither
// `Length` nor `Chars` is greater than 0, or if either is larger than 1048576.
// It is safe for concurrent use.
func String(opts *Options) (string, error) {

	if opts == nil {
		opts = DefaultOptions()
	}

	if opts.Length < 0 || opts.Length > max_length {
		return "", fmt.Errorf("invalid length: %d", opts.Length)
	}

	if opts.Chars < 0 || opts.Chars > max_length {
		return "", fmt.Errorf("invalid chars: %d", opts.Chars)
	}

	if opts.Length == 0 && opts.Chars == 0 {
		return "", errors.New("length or chars must be greater than 0")
	}

	if opts.Emoji && (opts.ASCII || opts.AlphaNumeric) {
		return "", errors.New("emoji can not be combined with ascii or alphanumeric")
	}

	if opts.Emoji && opts.Chars == 0 && opts.Length < 4 {
		return "", fmt.Errorf("invalid length for emoji: %d", opts.Length)
	}

	pool := runes

	if opts.Emoji {
		pool = emoji_runes
	} else if opts.AlphaNumeric {
		pool = alpha_numeric_runes
	} else if opts.ASCII {
		pool = ascii_runes
	}

	count := len(pool)

	src := newRandSource()

	var result strings.Builder

	if opts.Chars > 0 {

		result.Grow(opts.Chars)

		for i := 0; i < opts.Chars; i++ {

			j, err := src.intn(count)

			if err != nil {
				return "", err
			}

			result.WriteRune(pool[j])
		}

	} else {

		result.Grow(opts.Length)

		b := 0

		for b < opts.Length {

			if opts.Emoji && opts.Length-b < 4 {
				break
			}

			j, err := src.intn(count)

			if err != nil {
				return "", err
			}

			r := pool[j]
			l := utf8.RuneLen(r)

			// Redraw if the character would overshoot the requested length
			if b+l > opts.Length {
				continue
			}

			result.WriteRune(r)
			b += l
		}
	}

	s := result.String()

	if opts.Base32 {
		s = base32.StdEncoding.EncodeToString([]byte(s))
	}

	return s, nil
}
