package profiledefinition

type ProfileSource string

const (
	SourceCustom  ProfileSource = "custom"
	SourceDefault ProfileSource = "default"
)

// ProfileBundleProfileMetadata contains device profile metadata for downloaded profiles.
type ProfileBundleProfileMetadata struct {
	Source ProfileSource `json:"source"`
}

// ProfileBundleProfileItem represent a profile entry with metadata.
type ProfileBundleProfileItem struct {
	Metadata ProfileBundleProfileMetadata `json:"metadata"`
	Profile  ProfileDefinition            `json:"profile"`
}

// ProfileBundleResponse represent a list of profiles meant to be downloaded by user.
type ProfileBundleResponse struct {
	Time     string                     `json:"time"` // datetime when the bundle has been created
	Profiles []ProfileBundleProfileItem `json:"profiles"`
}
