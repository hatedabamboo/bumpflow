package main

import (
	"regexp"
	"strconv"
	"strings"
)

var semverRe = regexp.MustCompile(`^v?\d+(?:\.\d+)?`)

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		nums[i] = n
	}
	return nums, true
}

func versionGreater(a, b string) bool {
	va, ok1 := parseVersion(a)
	vb, ok2 := parseVersion(b)
	if !ok1 || !ok2 {
		return false
	}
	length := max(len(va), len(vb))
	for i := range length {
		ai, bi := 0, 0
		if i < len(va) {
			ai = va[i]
		}
		if i < len(vb) {
			bi = vb[i]
		}
		if ai != bi {
			return ai > bi
		}
	}
	return false
}
