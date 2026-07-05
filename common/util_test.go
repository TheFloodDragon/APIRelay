package common

import "testing"

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"short", "***"},
		{"12345678", "***"},
		{"123456789", "1234...6789"},
		{"sk-abcdefghijklmnopqrstuvwxyz", "sk-a...wxyz"},
	}
	for _, tc := range cases {
		if got := MaskSecret(tc.in); got != tc.want {
			t.Fatalf("MaskSecret(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
