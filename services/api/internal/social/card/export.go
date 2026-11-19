package card

import "html/template"

// FontCSS returns the embedded @font-face rules (AF Sandarenu, Noto Sans Sinhala, Abhaya Libre,
// Yaldevi Colombo) so other renderers can use the same brand type.
func FontCSS() template.CSS { return fontCSS }

// Masthead returns the Thulawa masthead PNG (2856×450) as a data: URL.
func Masthead() template.URL { return masthead }
