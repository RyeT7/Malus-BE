package httpx

import "testing"

func TestETagMatches(t *testing.T) {
	const etag = `"abc"`
	cases := map[string]bool{
		"":                 false,
		`"abc"`:            true,
		`W/"abc"`:          true,
		`"xyz", "abc"`:     true,
		`"xyz"`:            false,
		"*":                true,
		`"abcd"`:           false,
		` "xyz" , W/"abc"`: true,
	}
	for header, want := range cases {
		if got := ETagMatches(header, etag); got != want {
			t.Errorf("ETagMatches(%q) = %v, want %v", header, got, want)
		}
	}
}
