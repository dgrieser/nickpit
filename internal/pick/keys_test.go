package pick

import "testing"

func TestDecodeKeys(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		more     bool
		want     key
		consumed int
	}{
		{"up arrow", "\x1b[A", true, key{kind: keyUp}, 3},
		{"down arrow", "\x1b[B", true, key{kind: keyDown}, 3},
		{"application mode up", "\x1bOA", true, key{kind: keyUp}, 3},
		{"page up", "\x1b[5~", true, key{kind: keyPageUp}, 4},
		{"page down", "\x1b[6~", true, key{kind: keyPageDown}, 4},
		{"home", "\x1b[H", true, key{kind: keyHome}, 3},
		{"end", "\x1b[F", true, key{kind: keyEnd}, 3},
		{"modified arrow", "\x1b[1;5A", true, key{kind: keyUp}, 6},
		{"enter", "\r", true, key{kind: keyEnter}, 1},
		{"newline", "\n", true, key{kind: keyEnter}, 1},
		{"backspace", "\x7f", true, key{kind: keyBackspace}, 1},
		{"ctrl-c", "\x03", true, key{kind: keyAbort}, 1},
		{"ctrl-d", "\x04", true, key{kind: keyAbort}, 1},
		{"ctrl-n", "\x0e", true, key{kind: keyDown}, 1},
		{"ctrl-p", "\x10", true, key{kind: keyUp}, 1},
		{"ctrl-u", "\x15", true, key{kind: keyClearFilter}, 1},
		{"rune", "q", true, key{kind: keyRune, rune: 'q'}, 1},
		{"multibyte rune", "ü", true, key{kind: keyRune, rune: 'ü'}, 2},
		// A key the list does not use is consumed, never fed to the filter.
		{"unknown csi", "\x1b[3~", true, key{}, 4},
		{"alt key", "\x1bx", true, key{}, 2},
		{"other control", "\x01", true, key{}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, consumed := decode([]byte(tc.in), tc.more)
			if got != tc.want || consumed != tc.consumed {
				t.Fatalf("decode(%q) = %+v, %d; want %+v, %d", tc.in, got, consumed, tc.want, tc.consumed)
			}
		})
	}
}

// An escape sequence can arrive split across reads; deciding too early would
// turn an arrow key into an abort or leak "[A" into the filter.
func TestDecodeWaitsForSplitSequences(t *testing.T) {
	for _, partial := range []string{"\x1b", "\x1b[", "\x1b[1;"} {
		if got, consumed := decode([]byte(partial), true); consumed != 0 {
			t.Fatalf("decode(%q) = %+v, %d; want it to wait for more input", partial, got, consumed)
		}
	}
	// A partial multibyte rune waits the same way.
	if _, consumed := decode([]byte("ü")[:1], true); consumed != 0 {
		t.Fatal("a split rune must wait for its continuation byte")
	}
}

// With no continuation possible, a trailing ESC is the abort key.
func TestDecodeLoneEscapeAborts(t *testing.T) {
	got, consumed := decode([]byte("\x1b"), false)
	if got.kind != keyAbort || consumed != 1 {
		t.Fatalf("decode(ESC, more=false) = %+v, %d; want abort, 1", got, consumed)
	}
}

func TestDecodeEmpty(t *testing.T) {
	if _, consumed := decode(nil, true); consumed != 0 {
		t.Fatal("an empty buffer decodes nothing")
	}
}
