// Package sanitize removes or replaces problematic Unicode characters from
// strings and provides strict parsing helpers for numeric input.
//
// It is a Go port of the sanitizing routines in Flamework's lib_sanitize.php.
// Sanitizing a string involves, in order:
//
//  1. rejecting input that is not valid UTF-8,
//  2. removing "evil" code points (most control characters, byte order marks,
//     bidirectional controls, zero width spaces, Unicode "tag" characters),
//  3. removing either all "other" (\p{C}) characters or only invalid
//     non-characters, depending on [Options.StripReserved],
//  4. normalizing tabs, line breaks, form feeds and replacement characters.
//
// Basic usage:
//
//	opts := sanitize.DefaultOptions()
//	clean, err := sanitize.SanitizeString(userInput, opts)
//
// Useful references:
//
//   - https://github.com/exflickr/flamework/blob/master/www/include/lib_sanitize.php
//   - https://blog.golang.org/strings
//   - https://golang.org/pkg/regexp/syntax/
//   - http://www.fileformat.info/info/unicode/char/search.htm
//   - http://www.regular-expressions.info/unicode.html
package sanitize

import (
	"errors"
	_ "log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Package-level patterns, compiled once in init so SanitizeString does not
// pay the compile cost for them on every call.
var (
	// re_evil matches code points that are never acceptable in sanitized text.
	re_evil *regexp.Regexp
	// re_reserved matches the whole Unicode "Other" (\p{C}) category; used
	// only when Options.StripReserved is set.
	re_reserved *regexp.Regexp
	// re_invalid matches non-character sequences (e.g. U+FFFE/U+FFFF and
	// U+FDD0..U+FDEF); used when StripReserved is not set.
	re_invalid *regexp.Regexp
	// re_linefeeds matches tabs, line breaks and replacement characters that
	// SanitizeString maps to configurable replacements.
	re_linefeeds *regexp.Regexp
)

// init builds the package's regular expressions.
//
// NOTE: Go's regexp package matches on code points, not raw bytes, so an
// escape such as \xC2 means U+00C2 rather than the byte 0xC2. The byte-oriented
// patterns in lib_sanitize therefore don't translate directly; the patterns
// below are written in terms of code points instead (\x{....}).
func init() {

	/*

		evil codepoints, lib_sanitize says:

		U+0000..U+0008		00000000..00001000				\x00..\x08				[\x00-\x08]
		U+000E..U+001F		00001110..00011111				\x0E..\x1F				[\x0E-\x1F]
		U+007F..U+0084		01111111..10000100				\x7F,\xC2\x80..\xC2\x84			\x7F|\xC2[\x80-\x84\x86-\x9F]
		U+0086..U+009F		10000110..10011111				\xC2\x86..\xC2\x9F			^see above^

		go-sanitize says we are actually doing this for "U+0086..U+009F" (20160909/thisisaaronland)

		"\\x7F",
		"(?:\\x7F|\\xC2)?[\\x80-\\x84\\x86-\\x9F]",

		lib_sanitize goes on to say:

		U+FEFF			1111111011111111				\xEF\xBB\xBF				\xEF\xBB\xBF

		go-sanitize says we are actually doing this for "U+FEFF" because the pattern in lib_sanitize
		does not seem to work... (20160909/thisisaaronland)

		"\x{FEFF}"

		lib_sanitize goes on to say:

		U+206A..U+206F		10000001101010..10000001101111			\xE2\x81\xAA..\xE2\x81\xAF		\xE2\x81[\xAA-\xAF]
		U+FFF9..U+FFFA		1111111111111001..1111111111111010		\xEF\xBF\xB9..\xEF\xBF\xBA		\xEF\xBF[\xB9-\xBA]

		go-sanitize says we are actually doing this for "U+FFF9" because the pattern in lib_sanitize
		does not seem to work... (20160909/thisisaaronland)

		"\x{FFF9}"

		lib_sanitize goes on to say:

		U+E0000..U+E007F	11100000000000000000..11100000000001111111	\xF3\xA0\x80\x80..\xF3\xA0\x81\xBF	\xF3\xA0[\x80-\x81][\x80-\xBF]

		go-sanitize says we aren't able to match '11100000000000000000 U+E0000 '\U000e0000'  f3  a0  80  80'
		through '11100000000001111110 U+E007E '\U000e007e'  f3  a0  81  be' because... again, I am not really
		sure but we are brute-forcing the issue by assigning each individual codepoint from the Tags block
		below because apparently Go doesn't like "\x{FOO} - \x{BAR}" ranges because ... computers, I guess
		(20160909/thisisaaronland)

		lib_sanitize goes on to say:

		U+D800..U+DFFF		1101100000000000..1101111111111111		\xED\xA0\x80..\xED\xBF\xBF		\xED[\xA0-\xBF][\x80-\xBF]
		U+110000..U+13FFFF	100010000000000000000..100111111111111111111	\xf4\x90\x80\x80..\xf4\xbf\xbf\xbf	\xf4[\x90-\xbf][\x80-\xbf][\x80-\xbf]

	*/

	/*

		go-sanitize says, you might be wondering "what is all this stuff anyway..."

		U+0000..U+0008
		http://www.fileformat.info/info/unicode/category/Cc/index.htm
		http://www.fileformat.info/info/unicode/block/basic_latin/index.htm

		U+000E..U+001F
		http://www.fileformat.info/info/unicode/category/Cc/index.htm
		http://www.fileformat.info/info/unicode/block/basic_latin/index.htm

		U+007F..U+0084
		http://www.fileformat.info/info/unicode/category/Cc/index.htm
		http://www.fileformat.info/info/unicode/block/basic_latin/index.htm

		U+0086..U+009F
		http://www.fileformat.info/info/unicode/category/Cc/index.htm
		http://www.fileformat.info/info/unicode/block/latin_supplement/index.htm

		U+FEFF - your old friend the "ZERO WIDTH NO-BREAK SPACE" or "BYTE ORDER MARK"
		http://www.fileformat.info/info/unicode/char/feff/index.htm

		U+206A..U+206F
		http://www.fileformat.info/info/unicode/category/Cf/index.htm
		http://www.fileformat.info/info/unicode/block/general_punctuation/index.htm

		U+FFF9..U+FFFA
		http://www.fileformat.info/info/unicode/category/Cf/index.htm
		http://www.fileformat.info/info/unicode/block/specials/index.htm

		U+E0000..U+E007F
		http://www.fileformat.info/info/unicode/category/Cf/index.htm
		http://www.fileformat.info/info/unicode/block/tags/index.htm
		https://en.wikipedia.org/wiki/Tags_(Unicode_block) <-- just stop a take a moment to let this one sink in...

		U+D800..U+DFFF
		http://www.fileformat.info/info/unicode/block/high_surrogates/index.htm
		http://www.fileformat.info/info/unicode/block/low_surrogates/index.htm

		U+110000..U+13FFFF

	*/

	// Each entry is a regexp alternative; they are joined with "|" below.
	evil_codepoints := []string{
		"[\\x00-\\x08]",                    // C0 controls before TAB
		"[\\x0E-\\x1F]",                    // C0 controls after CR (TAB, LF, VT, FF, CR are handled separately)
		"\\x7F",                            // DEL
		"[\\x{80}-\\x{84}\\x{86}-\\x{9F}]", // C1 controls; U+0085 (NEL) is handled as a line break below
		"\\x{FEFF}",                        // BOM / zero width no-break space
		"[\\x{206A}-\\x{206F}]",            // deprecated format controls
		// Bidirectional controls enable "Trojan Source" and filename spoofing.
		"[\\x{200E}\\x{200F}\\x{202A}-\\x{202E}\\x{2066}-\\x{2069}]",
		// Invisible characters: zero width space and word joiner. ZWNJ/ZWJ
		// (U+200C/U+200D) are kept because some scripts and emoji sequences need them.
		"[\\x{200B}\\x{2060}]",
		"[\\x{FFF9}-\\x{FFFA}]",   // interlinear annotation anchor/separator
		"[\\x{E0000}-\\x{E007F}]", // Tags block
		// Surrogates (U+D800..U+DFFF) and code points above U+10FFFF cannot
		// appear in valid UTF-8, which SanitizeString checks before matching.
	}

	// Combine every alternative into a single pattern.
	re_evil = regexp.MustCompile(strings.Join(evil_codepoints, "|"))

	/*

		reserved characters

		lib_sanitize says \p{Cn} as in \p{Unassigned} however Go recognized neither of those as
		valid character class ranges so we have to settle for the entirety of the \p{Other}
		range which is inclusive of things like control character (trapped above) and private
		use characters and so on... computers, right? (20160909/thisisaaronland)

	*/

	re_reserved = regexp.MustCompile(`\p{C}`)

	/*

		invalid characters, as in:

		s := "\xF0\xBF"
		r,_ := utf8.DecodeRuneInString(s)
		fmt.Printf("%+q\n", r)
		fmt.Printf("%.8b\n", r)

		1111111111111101	\ufffd		F4 8F (and so on...)

	*/

	// Matches the Unicode "noncharacters": U+FDD0..U+FDEF and the last two
	// code points (xFFFE, xFFFF) of each of the 17 planes.
	re_invalid = regexp.MustCompile(`[\x{FDD0}-\x{FDEF}]|[\x{FFFE}\x{FFFF}\x{1FFFE}\x{1FFFF}\x{2FFFE}\x{2FFFF}\x{3FFFE}\x{3FFFF}\x{4FFFE}\x{4FFFF}\x{5FFFE}\x{5FFFF}\x{6FFFE}\x{6FFFF}\x{7FFFE}\x{7FFFF}\x{8FFFE}\x{8FFFF}\x{9FFFE}\x{9FFFF}\x{AFFFE}\x{AFFFF}\x{BFFFE}\x{BFFFF}\x{CFFFE}\x{CFFFF}\x{DFFFE}\x{DFFFF}\x{EFFFE}\x{EFFFF}\x{FFFFE}\x{FFFFF}\x{10FFFE}\x{10FFFF}]`)

	// The set of line-break style characters is fixed, so the pattern is
	// compiled once; only their replacements vary with Options. "\r\n" is
	// listed first so a CRLF is matched as one break rather than two.
	re_linefeeds = regexp.MustCompile("\r\n|\u2028|\u2029|\u0085|\t|\v|\f|\r|\n|\uFFFC|\uFFFD")
}

// Options controls how SanitizeString behaves.
//
// The replacement fields are unexported; obtain a configured value from
// DefaultOptions or DebugOptions and then adjust the exported fields.
type Options struct {
	// StripReserved removes every character in the Unicode "Other" category
	// (\p{C}: controls, format, private use, unassigned, surrogates). When
	// false, only invalid non-characters are removed.
	StripReserved bool
	// AllowNewlines preserves line breaks as "\n" (and form feeds / paragraph
	// separators as "\n\n") instead of replacing them.
	AllowNewlines bool
	// replacementString replaces removed "evil", reserved and invalid characters.
	replacementString string
	// replacementTab replaces horizontal tabs.
	replacementTab string
	// replacementLF replaces line breaks (LF, CR, CRLF, NEL, U+2028).
	replacementLF string
	// replacementFF replaces form feed, vertical tab and U+2029.
	replacementFF string
	// replacementUnknown replaces U+FFFC (object replacement) and U+FFFD
	// (replacement character).
	replacementUnknown string
}

// DefaultOptions returns Options that delete unsafe characters and turn tabs
// and line breaks into single spaces. Unknown/replacement characters become "?".
func DefaultOptions() *Options {

	o := Options{
		StripReserved:      false,
		AllowNewlines:      false,
		replacementString:  "",
		replacementLF:      " ",
		replacementFF:      " ",
		replacementTab:     " ",
		replacementUnknown: "?",
	}

	return &o
}

// DebugOptions returns Options that replace every sanitized character with the
// visible marker " { SANITIZED } ", making it easy to see what was altered.
func DebugOptions() *Options {

	o := Options{
		StripReserved:      false,
		AllowNewlines:      false,
		replacementString:  " { SANITIZED } ",
		replacementLF:      " { SANITIZED } ",
		replacementFF:      " { SANITIZED } ",
		replacementTab:     " { SANITIZED } ",
		replacementUnknown: " { SANITIZED } ",
	}

	return &o
}

// SanitizeString returns input with unsafe or invisible Unicode characters
// removed or replaced according to options.
//
// It returns an error, and an empty string, if input is not valid UTF-8.
// If options is nil, DefaultOptions is used.
func SanitizeString(input string, options *Options) (string, error) {

	if options == nil {
		options = DefaultOptions()
	}

	// Invalid UTF-8 can't be reasoned about by the code-point patterns below.
	if !utf8.ValidString(input) {
		return "", errors.New("Invalid UTF8 string")
	}

	output := input

	// Step 1: strip control characters, BOMs, format controls, tags, etc.
	output = re_evil.ReplaceAllString(output, options.replacementString)

	// Step 2: strip either all "Other" characters or only non-characters.
	if options.StripReserved {

		output = re_reserved.ReplaceAllString(output, options.replacementString)

	} else {

		output = re_invalid.ReplaceAllString(output, options.replacementString)
	}

	// Step 3: choose what line breaks / form feeds become.
	lf := options.replacementLF
	ff := options.replacementFF

	if options.AllowNewlines {
		lf = "\n"
		ff = "\n\n"
	}

	// Map each whitespace-like character to its replacement.
	lookup := map[string]string{
		"\xE2\x80\xA8": lf, // U+2028
		"\xE2\x80\xA9": ff, // U+2029
		"\xC2\x85":     lf, // EBCDIC Next Line / NEL
		"\x09":         options.replacementTab,
		"\x0B":         ff,
		"\x0C":         ff,
		"\r\n":         lf,
		"\r":           lf,
		"\n":           lf,
		"\xEF\xBF\xBC": options.replacementUnknown, // U+FFFC
		"\xEF\xBF\xBD": options.replacementUnknown, // U+FFFD
	}

	// Each match is swapped for its mapped replacement.
	cb_linefeeds := func(s string) string {
		return lookup[s]
	}

	output = re_linefeeds.ReplaceAllStringFunc(output, cb_linefeeds)

	return output, nil
}

// SanitizeInt32 parses input as a base-10 integer that fits in an int32.
// It returns the strconv error (syntax or range) if parsing fails.
func SanitizeInt32(input string) (int32, error) {

	output, err := strconv.ParseInt(input, 10, 32)

	if err != nil {
		return 0, err
	}

	return int32(output), nil
}

// SanitizeInt64 parses input as a base-10 integer that fits in an int64.
// It returns the strconv error (syntax or range) if parsing fails.
func SanitizeInt64(input string) (int64, error) {

	return strconv.ParseInt(input, 10, 64)
}

// SanitizeFloat64 parses input as a 64-bit floating point number.
// It returns the strconv error (syntax or range) if parsing fails, and an
// error for NaN and infinite values, which strconv otherwise accepts.
func SanitizeFloat64(input string) (float64, error) {

	output, err := strconv.ParseFloat(input, 64)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(output) || math.IsInf(output, 0) {
		return 0, errors.New("Value is not a finite number")
	}

	return output, nil
}
