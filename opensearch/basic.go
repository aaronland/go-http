package opensearch

// BasicOpenSearchDescriptionOptions defines the options for creating a basic `OpenSearchDescription`.
type BasicOpenSearchDescriptionOptions struct {
	// Name is the short name of the search engine.
	Name string
	// Description is a brief description of the search engine.
	Description string
	// QueryParameter is the name of the URL parameter that carries the search terms.
	QueryParameter string
	// SearchTemplate is the URL template used to perform searches.
	SearchTemplate string
	// SearchForm is the URL of a web page containing a search form for the search engine.
	SearchForm string
	// ImageURI is the URI (typically a data URI) of the search engine's icon.
	ImageURI string
}

// BasicOpenSearchDescription returns a new `OpenSearchDescription` derived from 'opts' using default values for
// the icon dimensions, URL type and method, and search terms placeholder.
func BasicOpenSearchDescription(opts *BasicOpenSearchDescriptionOptions) (*OpenSearchDescription, error) {

	im := &OpenSearchImage{
		Height: DEFAULT_IMAGE_HEIGHT,
		Width:  DEFAULT_IMAGE_WIDTH,
		URI:    opts.ImageURI,
	}

	params := []*OpenSearchURLParameter{
		&OpenSearchURLParameter{
			Name:  opts.QueryParameter,
			Value: DEFAULT_SEARCHTERMS,
		},
	}

	u := &OpenSearchURL{
		Type:       DEFAULT_URL_TYPE,
		Method:     DEFAULT_URL_METHOD,
		Template:   opts.SearchTemplate,
		Parameters: params,
	}

	desc := &OpenSearchDescription{
		NSMoz:         NS_MOZILLA_SEARCH,
		NSOpenSearch:  NS_OPENSEARCH,
		InputEncoding: "UTF-8",
		ShortName:     opts.Name,
		Description:   opts.Description,
		Image:         im,
		URL:           u,
		SearchForm:    opts.SearchForm,
	}

	return desc, nil
}
