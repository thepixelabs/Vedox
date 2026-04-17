package docgraph

// internal_test.go — white-box tests for helpers that are not exported.

import "testing"

// TestEscapeLikePrefix is a regression test for a bug where a project id
// containing SQLite LIKE metacharacters (%, _) would accidentally match
// siblings. The fix escapes those characters using the ESCAPE '\' clause
// already present on the query.
func TestEscapeLikePrefix(t *testing.T) {
	cases := map[string]string{
		"foo/":          "foo/",
		"foo_bar/":      `foo\_bar/`,
		"foo%bar/":      `foo\%bar/`,
		`foo\bar/`:      `foo\\bar/`,
		"a_b%c\\d":      `a\_b\%c\\d`,
		"":              "",
	}
	for in, want := range cases {
		if got := escapeLikePrefix(in); got != want {
			t.Errorf("escapeLikePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
