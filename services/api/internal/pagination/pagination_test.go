package pagination

import "testing"

func TestNewMeta(t *testing.T) {
	tests := []struct {
		name       string
		total      int
		totalPages int
	}{
		{name: "empty", totalPages: 1},
		{name: "partial page", total: 11, totalPages: 2},
		{name: "full pages", total: 20, totalPages: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta := NewMeta(Params{Page: 1, PerPage: 10}, test.total)
			if meta.TotalPages != test.totalPages {
				t.Fatalf("TotalPages = %d, want %d", meta.TotalPages, test.totalPages)
			}
		})
	}
}
