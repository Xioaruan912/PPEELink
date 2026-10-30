// Package agentversion defines the Agent release independently of panel builds.
package agentversion

import (
	"fmt"
	"strconv"
	"strings"
)

// Current starts the new release sequence. Agent changes increment the second
// integer: 1.11 -> 1.12 -> ... -> 1.19 -> 1.20. Panel-only changes do not bump it.
const Current = "1.12"

func Parse(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "v"), ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	for _, p := range parts {
		if p == "" {
			return 0, 0, false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return 0, 0, false
			}
		}
	}
	major, e1 := strconv.Atoi(parts[0])
	revision, e2 := strconv.Atoi(parts[1])
	return major, revision, e1 == nil && e2 == nil && major > 0 && revision >= 0
}
func Compare(left, right string) (int, bool) {
	lm, lr, ok := Parse(left)
	rm, rr, other := Parse(right)
	if !ok || !other {
		return 0, false
	}
	if lm < rm || (lm == rm && lr < rr) {
		return -1, true
	}
	if lm > rm || (lm == rm && lr > rr) {
		return 1, true
	}
	return 0, true
}
func Next(raw string) (string, error) {
	major, revision, ok := Parse(raw)
	if !ok || revision == int(^uint(0)>>1) {
		return "", fmt.Errorf("invalid Agent version")
	}
	return fmt.Sprintf("%d.%d", major, revision+1), nil
}
