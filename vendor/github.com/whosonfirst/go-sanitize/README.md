# go-sanitize

A Go port of the Flamework lib_sanitize library.

## What does it do?

Text that comes from the outside world (web forms, uploaded files, copy-and-paste) can contain characters you can't see but that cause real trouble: hidden control codes, invisible spaces, markers that flip the direction of text, and bytes that aren't valid text at all. These can garble your data, break other programs, or be used to disguise what a piece of text really says.

`go-sanitize` cleans that up. Give it a piece of text and it hands back a tidy version:

* **Invalid text is rejected.** If the input isn't valid UTF-8, you get an error instead of guesswork.
* **Hidden and troublemaking characters are removed.** This covers control codes, the byte order mark, text-direction overrides, zero-width spaces and similar invisible characters.
* **Tabs and line breaks are made consistent.** By default they become plain spaces. You can choose to keep line breaks.
* **Placeholder "unknown" characters become `?`.**
* **Optionally, strip even more.** Turn on `StripReserved` to also remove private-use and other unassigned characters.

It also has small helpers, `SanitizeInt32`, `SanitizeInt64` and `SanitizeFloat64`, that turn text into numbers and return an error if the text isn't a clean number. Not-a-number and infinity values are rejected.

### What it does *not* do

This package cleans up *characters*. It does not make text safe to put into a web page, database query or command line. For that you still need the right escaping for where the text is going, such as HTML escaping or parameterized SQL queries.

## Install

You will need to have both `Go` (specifically [version 1.12](https://golang.org/dl/) or higher because we're using [Go modules](https://github.com/golang/go/wiki/Modules)) and the `make` programs installed on your computer. 

All of this package's dependencies are bundled with the code in the `vendor` directory.

## Example

```
package main

import (
       "fmt"
       "github.com/whosonfirst/go-sanitize"
       )

func main() {

     input := "foo bar\nbaz ok:'\u2318' bad:'\u0007' bad:'\u206E' woop woop"
     
     opts := sanitize.DefaultOptions()
     output, _ := sanitize.SanitizeString(input, opts)

     fmt.Println(output)

}

```     

In this example the bell character (`\u0007`) and the deprecated format character (`\u206E`) are removed, the newline becomes a space, and the `⌘` symbol is kept.

## See also

* https://github.com/exflickr/flamework/blob/master/www/include/lib_sanitize.php
