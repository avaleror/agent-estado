package pass

import "strings"

const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

func parseID(raw string) (nameplate, password string, err error) {
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		switch r {
		case ' ', '-', '\n', '\r', '\t':
			continue
		case 'i', 'l':
			b.WriteByte('1')
		case 'o':
			b.WriteByte('0')
		default:
			if !strings.ContainsRune(alphabet, r) {
				return "", "", exitCode(1, "that id is not valid.")
			}
			b.WriteRune(r)
		}
	}
	if b.Len() != 34 {
		return "", "", exitCode(1, "that id is not valid.")
	}
	s := b.String()
	return s[:8], s[8:], nil
}

func validPlate(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(alphabet, r) {
			return false
		}
	}
	return true
}
