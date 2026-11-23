package service

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// normalizeEmail lower-cases and trims an email so lookups are case-insensitive.
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validEmail(s string) bool { return len(s) <= 254 && emailRe.MatchString(s) }

// checkLen validates the rune length of a trimmed string and records an error in fields.
func checkLen(fields map[string]string, field, value string, min, max int) {
	n := utf8.RuneCountInString(value)
	switch {
	case n < min && min == 1:
		fields[field] = "is required"
	case n < min:
		fields[field] = "must be at least " + strconv.Itoa(min) + " characters"
	case n > max:
		fields[field] = "must be at most " + strconv.Itoa(max) + " characters"
	}
}

// pagination normalises page / page_size, defaulting to page 1 and 20 items (max 100).
func pagination(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
