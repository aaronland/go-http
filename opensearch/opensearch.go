// Package opensearch provides types and HTTP handlers for publishing OpenSearch description documents
// and advertising them to browsers.
package opensearch

import (
	"encoding/xml"
)

// OpenSearchImage describes the icon of a search engine.
type OpenSearchImage struct {
	// Height is the height of the image in pixels.
	Height int `xml:"height,attr"`
	// Width is the width of the image in pixels.
	Width int `xml:"width,attr"`
	// URI is the URI of the image.
	URI string `xml:",chardata"`
}

// OpenSearchURL describes a URL template used to perform searches.
type OpenSearchURL struct {
	// Type is the MIME type of the search results.
	Type string `xml:"type,attr"`
	// Method is the HTTP method used to perform searches.
	Method string `xml:"method,attr"`
	// Template is the URL template for searches.
	Template string `xml:"template,attr"`
	// Parameters are the parameters passed with the search request.
	Parameters []*OpenSearchURLParameter `xml:"Param"`
}

// OpenSearchURLParameter is a name and value pair passed with a search request.
type OpenSearchURLParameter struct {
	// Name is the parameter name.
	Name string `xml:"name,attr"`
	// Value is the parameter value, which may contain OpenSearch placeholders.
	Value string `xml:"value,attr"`
}

// OpenSearchDescription is an OpenSearch description document.
type OpenSearchDescription struct {
	XMLName       xml.Name         `xml:"OpenSearchDescription"`
	NSMoz         string           `xml:"xmlns:moz,attr"`
	InputEncoding string           `xml:"InputEncoding"`
	NSOpenSearch  string           `xml:"xmlns,attr"`
	ShortName     string           `xml:"ShortName"`
	Description   string           `xml:"Description"`
	Image         *OpenSearchImage `xml:"Image"`
	URL           *OpenSearchURL   `xml:"Url"`
	SearchForm    string           `xml:"moz:searchForm"`
}

// Marshal returns the XML encoding of 'd', including the XML header.
func (d *OpenSearchDescription) Marshal() ([]byte, error) {

	enc, err := xml.Marshal(d)

	if err != nil {
		return nil, err
	}

	body := []byte(xml.Header)
	body = append(body, enc...)

	return body, nil
}
