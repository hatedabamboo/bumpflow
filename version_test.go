package main

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input  string
		want   []int
		wantOk bool
	}{
		{"v1.2.3", []int{1, 2, 3}, true},
		{"1.2.3", []int{1, 2, 3}, true},
		{"v1.0", []int{1, 0}, true},
		{"1", []int{1}, true},
		{"v1.x.3", nil, false},
		{"abc", nil, false},
		{"", nil, false},
	}
	for _, tt := range tests {
		got, ok := parseVersion(tt.input)
		if ok != tt.wantOk {
			t.Errorf("parseVersion(%q) ok=%v, want %v", tt.input, ok, tt.wantOk)
			continue
		}
		if !ok {
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseVersion(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestVersionGreater(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v2.0.0", "v1.0.0", true},
		{"v1.0.0", "v2.0.0", false},
		{"v1.2.3", "v1.2.3", false},
		{"v1.10.0", "v1.9.0", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0", "v1.0.0", false},
		{"v1.0.1", "v1.0", true},
		{"abc", "v1.0", false},
		{"v1.0", "abc", false},
	}
	for _, tt := range tests {
		got := versionGreater(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("versionGreater(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
