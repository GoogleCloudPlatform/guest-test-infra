package gceimgresource

// Source is the configuration specifying which images to return.
type Source struct {
	Project string `json:"project"`
	// Family limits matching images to those with the specified family. Optional.
	Family string `json:"family"`
	// Regexp defines a regular expression to find versions embedded in image names. Optional.
	Regexp string `json:"regexp"` // TODO: Not yet implemented.
	// ReadyOnly filters images which are in READY status. Optional.
	ReadyOnly bool `json:"readyOnly"`
}

// Version represents a single image version.
type Version struct {
	// Name is used to determine the image in a `get` step.
	Name string `json:"name"`
	// Version is used for ordering returned images in `check` steps. Defaults to the image creation timestamp.
	// If regexp is specified, Version will instead contain the parsed version.
	Version string `json:"version,omitempty"`
}
