// Package webfinger defines types and constants for WebFinger (RFC 7033) responses.
package webfinger

// https://www.rfc-editor.org/rfc/rfc7033 (webfinger)
// https://www.rfc-editor.org/rfc/rfc7565 (acct:)

// ContentType is the media type of WebFinger (JRD) responses.
const ContentType string = "application/jrd+json"

// Endpoint is the well-known URL path for WebFinger requests.
const Endpoint string = "/.well-known/webfinger"

// Resource is a WebFinger JSON Resource Descriptor (JRD).
type Resource struct {
	Subject    string            `json:"subject,omitempty"`
	Aliases    []string          `json:"aliases,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Links      []Link            `json:"links,omitempty"`
}

// Link is a link relation in a WebFinger `Resource`.
type Link struct {
	HRef       string            `json:"href"`
	Type       string            `json:"type,omitempty"`
	Rel        string            `json:"rel"`
	Properties map[string]string `json:"properties,omitempty"`
	Titles     map[string]string `json:"titles,omitempty"`
}
