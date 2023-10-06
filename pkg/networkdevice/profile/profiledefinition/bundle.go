package profiledefinition

// ProfileEntry represent a profile entry with metadata.
type ProfileEntry struct {
	// TODO: Metadata can be added here later
	Profile ProfileDefinition `json:"profile"`
}

// ProfileBundleResponse represent a list of profiles meant to be downloaded by user.
type ProfileBundleResponse struct {
	Time     string         `json:"time"` // datetime when the bundle has been created
	Profiles []ProfileEntry `json:"profiles"`
}
