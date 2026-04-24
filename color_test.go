package main

import "testing"

func TestClr(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()

	colorEnabled = false
	if got := clr(ansiRed, "hello"); got != "hello" {
		t.Errorf("clr with color disabled: got %q, want %q", got, "hello")
	}

	colorEnabled = true
	want := ansiRed + "hello" + ansiReset
	if got := clr(ansiRed, "hello"); got != want {
		t.Errorf("clr with color enabled: got %q, want %q", got, want)
	}
}

func TestColorHelpers(t *testing.T) {
	orig := colorEnabled
	colorEnabled = true
	defer func() { colorEnabled = orig }()

	tests := []struct {
		name string
		fn   func(string) string
		code string
	}{
		{"bold", bold, ansiBold},
		{"cRed", cRed, ansiRed},
		{"cGreen", cGreen, ansiGreen},
		{"cYellow", cYellow, ansiYellow},
		{"cCyan", cCyan, ansiCyan},
		{"cDim", cDim, ansiDim},
	}
	for _, tt := range tests {
		got := tt.fn("x")
		want := tt.code + "x" + ansiReset
		if got != want {
			t.Errorf("%s(%q) = %q, want %q", tt.name, "x", got, want)
		}
	}
}
