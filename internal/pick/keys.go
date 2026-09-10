package pick

import "unicode/utf8"

// keyKind classifies one decoded keypress. Everything the list reacts to has a
// kind; keys it ignores decode to keyOther so the read loop still consumes
// their bytes.
type keyKind int

const (
	keyOther keyKind = iota
	keyRune
	keyUp
	keyDown
	keyPageUp
	keyPageDown
	keyHome
	keyEnd
	keyEnter
	keyBackspace
	keyClearFilter
	keyAbort
)

// key is a decoded keypress; Rune is set only for keyRune.
type key struct {
	kind keyKind
	rune rune
}

// decode reads the first keypress from buf and reports how many bytes it
// consumed. A consumed count of 0 means buf holds the start of a longer
// sequence (an escape sequence split across reads, or a partial UTF-8 rune)
// and the caller must read more before decoding again.
//
// A lone ESC is the abort key, so it can only be decided once no continuation
// arrived: callers pass more=false to resolve a trailing ESC. Terminals emit a
// whole escape sequence in one write, so a real arrow key never resolves as
// abort in practice.
func decode(buf []byte, more bool) (key, int) {
	if len(buf) == 0 {
		return key{}, 0
	}
	switch buf[0] {
	case 0x1b:
		return decodeEscape(buf, more)
	case '\r', '\n':
		return key{kind: keyEnter}, 1
	case 0x7f, 0x08:
		return key{kind: keyBackspace}, 1
	case 0x03, 0x04: // Ctrl-C, Ctrl-D
		return key{kind: keyAbort}, 1
	case 0x0e: // Ctrl-N
		return key{kind: keyDown}, 1
	case 0x10: // Ctrl-P
		return key{kind: keyUp}, 1
	case 0x15: // Ctrl-U
		return key{kind: keyClearFilter}, 1
	}
	if buf[0] < 0x20 {
		return key{}, 1
	}
	r, size := utf8.DecodeRune(buf)
	if r == utf8.RuneError && size <= 1 {
		// Either an invalid byte (drop it) or a rune split across reads.
		if !utf8.FullRune(buf) && more {
			return key{}, 0
		}
		return key{}, 1
	}
	return key{kind: keyRune, rune: r}, size
}

// decodeEscape decodes the CSI and SS3 sequences the arrow, paging and
// home/end keys emit; anything else escape-introduced is consumed and ignored
// so an unknown sequence cannot leak into the filter as text.
func decodeEscape(buf []byte, more bool) (key, int) {
	if len(buf) == 1 {
		if more {
			return key{}, 0
		}
		return key{kind: keyAbort}, 1
	}
	if buf[1] != '[' && buf[1] != 'O' {
		// ESC followed by an ordinary key: Alt-<key>, which the list ignores.
		return key{}, 2
	}
	if len(buf) == 2 {
		return key{}, 0
	}
	// CSI parameter bytes ("1;5", "5") precede the final byte.
	end := 2
	for end < len(buf) && buf[end] >= 0x30 && buf[end] <= 0x3f {
		end++
	}
	if end >= len(buf) {
		return key{}, 0
	}
	params := string(buf[2:end])
	final := buf[end]
	consumed := end + 1
	switch final {
	case 'A':
		return key{kind: keyUp}, consumed
	case 'B':
		return key{kind: keyDown}, consumed
	case 'H':
		return key{kind: keyHome}, consumed
	case 'F':
		return key{kind: keyEnd}, consumed
	case '~':
		switch params {
		case "1", "7":
			return key{kind: keyHome}, consumed
		case "4", "8":
			return key{kind: keyEnd}, consumed
		case "5":
			return key{kind: keyPageUp}, consumed
		case "6":
			return key{kind: keyPageDown}, consumed
		}
	}
	return key{}, consumed
}
