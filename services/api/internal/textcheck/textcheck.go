// Package textcheck rejects machine-written news text that mixes in characters from unrelated
// scripts (for example Cyrillic or Chinese inside a Sinhala summary), which models sometimes
// produce and readers cannot read.
package textcheck

import (
	"fmt"
	"unicode"
)

// languageScripts are the scripts a story may be written in. Latin is always allowed alongside
// them for names, outlets and numbers; Common and Inherited cover digits, punctuation, spaces,
// combining marks and the zero-width joiners Sinhala conjuncts need.
var languageScripts = []*unicode.RangeTable{unicode.Sinhala, unicode.Tamil, unicode.Latin}

// Foreign returns the first character from a script that does not belong in text whose main
// script is Sinhala, Tamil or Latin, and whether one was found. Text whose main script is none
// of these (for example a summary written entirely in Chinese) is foreign as a whole.
func Foreign(text string) (rune, bool) {
	main := mainScript(text)
	if main == nil {
		for _, r := range text {
			if unicode.IsLetter(r) {
				return r, true
			}
		}
		return 0, false
	}
	for _, r := range text {
		if unicode.In(r, main, unicode.Latin, unicode.Common, unicode.Inherited) {
			continue
		}
		return r, true
	}
	return 0, false
}

// Check returns an error naming the first foreign character, for use in output validation.
func Check(field, text string) error {
	if r, found := Foreign(text); found {
		return fmt.Errorf("%s contains a character from an unexpected script: %q (U+%04X)", field, r, r)
	}
	return nil
}

// mainScript is the language script with the most letters; Latin counts only when no Sinhala
// or Tamil letter is present. It returns nil when letters of another script dominate.
func mainScript(text string) *unicode.RangeTable {
	counts := make([]int, len(languageScripts))
	other := 0
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		matched := false
		for i, table := range languageScripts {
			if unicode.Is(table, r) {
				counts[i]++
				matched = true
				break
			}
		}
		if !matched {
			other++
		}
	}
	best, bestCount := -1, 0
	for i, count := range counts[:2] {
		if count > bestCount {
			best, bestCount = i, count
		}
	}
	if best < 0 {
		if counts[2] == 0 || other > counts[2] {
			return nil
		}
		return unicode.Latin
	}
	if other > bestCount {
		return nil
	}
	return languageScripts[best]
}
