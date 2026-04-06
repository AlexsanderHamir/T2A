package repo

import (
	"reflect"
	"testing"
)

func TestParseFileMentions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []Mention
	}{
		{
			name: "path only",
			in:   `see @foo/bar.go there`,
			want: []Mention{{Path: "foo/bar.go", RawStart: 4, RawEnd: 15}},
		},
		{
			name: "path with range",
			in:   `@web/src/App.tsx(1-10)`,
			want: []Mention{{Path: "web/src/App.tsx", StartLine: 1, EndLine: 10, HasRange: true, RawStart: 0, RawEnd: 22}},
		},
		{
			name: "two mentions",
			in:   `@a.go(2-3) and @b.go`,
			want: []Mention{
				{Path: "a.go", StartLine: 2, EndLine: 3, HasRange: true, RawStart: 0, RawEnd: 10},
				{Path: "b.go", RawStart: 15, RawEnd: 20},
			},
		},
		{
			name: "filename with parentheses is path only",
			in:   `see @docs/file(name).md for details`,
			want: []Mention{{Path: "docs/file(name).md", RawStart: 4, RawEnd: 23}},
		},
		{
			name: "range-like segment inside filename is not parsed as range",
			in:   `use @docs/file(1-2).md next`,
			want: []Mention{{Path: "docs/file(1-2).md", RawStart: 4, RawEnd: 22}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ParseFileMentions(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseFileMentions(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}
