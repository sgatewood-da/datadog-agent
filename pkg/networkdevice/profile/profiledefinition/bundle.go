package profiledefinition

type ProfileSource string

const (
	SourceCustom  ProfileSource = "custom"
	SourceDefault ProfileSource = "default"
)

// ProfileMetadata contains device profile metadata for downloaded profiles.
type ProfileMetadata struct {
	Source ProfileSource `json:"source"`
}

// ProfileEntry represent a profile entry with metadata.
type ProfileEntry struct {
	Metadata ProfileMetadata   `json:"metadata"`
	Profile  ProfileDefinition `json:"profile"`
}

// ProfileBundleResponse represent a list of profiles meant to be downloaded by user.
type ProfileBundleResponse struct {
	Time     string         `json:"time"` // datetime when the bundle has been created
	Profiles []ProfileEntry `json:"profiles"`
}
