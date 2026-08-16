package sinhala

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func NFC(value string) string {
	return norm.NFC.String(strings.TrimSpace(value))
}

func Predominant(value string) bool {
	var sinhala, letters int
	for _, r := range value {
		if unicode.IsLetter(r) {
			letters++
			if r >= 0x0D80 && r <= 0x0DFF {
				sinhala++
			}
		}
	}
	if letters == 0 {
		return utf8.RuneCountInString(value) > 0
	}
	return float64(sinhala)/float64(letters) >= 0.4
}
