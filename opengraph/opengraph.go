package opengraph

// https://ogp.me/

// OpenGraph describes a struct containing OpenGraph metadata to pass down to
// templates and include as HTML <meta> tags
type OpenGraph struct {
	Type        string	`json:"type"`
	SiteName    string	`json:"site_name"`
	Title       string	`json:"title"`
	Description string	`json:"description"`
	Image       string	`json:"image"`
}
