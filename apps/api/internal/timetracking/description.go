package timetracking

import (
	"strings"
	"unicode/utf8"
)

const maximumDescriptionLength = 200

func normalizedDescription(value string) (string, bool) {
	description := strings.TrimSpace(value)
	return description, description != "" && utf8.RuneCountInString(description) <= maximumDescriptionLength
}
