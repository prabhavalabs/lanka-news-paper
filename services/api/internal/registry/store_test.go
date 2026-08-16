package registry

import "testing"

func TestValidateIconURL(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "https://publisher.example/icon.png"} {
		if err := validateIconURL(value); err != nil {
			t.Fatalf("validateIconURL(%q) returned %v", value, err)
		}
	}
	for _, value := range []string{"/icon.png", "http://publisher.example/icon.png", "https://user@publisher.example/icon.png"} {
		if err := validateIconURL(value); err == nil {
			t.Fatalf("validateIconURL(%q) accepted an unsafe URL", value)
		}
	}
}
