package baidu

import "fmt"

func errEmptyQuote(id string) error {
	return fmt.Errorf("%s: empty quote", id)
}

func errSuspiciousPrice(id string, price, bound float64, dir string) error {
	return fmt.Errorf("%s: suspicious price %.4f %s %.4f", id, price, dir, bound)
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
