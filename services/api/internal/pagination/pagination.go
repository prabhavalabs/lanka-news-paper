package pagination

// Params contains validated server-side table pagination values.
type Params struct {
	Page    int
	PerPage int
	Search  string
}

// Limit returns the SQL row limit.
func (params Params) Limit() int {
	return params.PerPage
}

// Offset returns the SQL row offset.
func (params Params) Offset() int {
	return (params.Page - 1) * params.PerPage
}

// Meta describes a page of results.
type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NewMeta builds response metadata for a result count.
func NewMeta(params Params, total int) Meta {
	totalPages := (total + params.PerPage - 1) / params.PerPage
	if totalPages == 0 {
		totalPages = 1
	}
	return Meta{
		Page:       params.Page,
		PerPage:    params.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

// Page is the shared response shape for admin data tables.
type Page[T any] struct {
	Items      []T  `json:"items"`
	Pagination Meta `json:"pagination"`
}
