package profiledefinition

// DownloadProfileMetadata contains device profile metadata for downloaded profiles.
type DownloadProfileMetadata struct {
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	Version   uint64 `json:"version"`
}

// DownloadProfileItem represent a profile.
type DownloadProfileItem struct {
	Profile  ProfileDefinition       `json:"profile,omitempty"`
	Metadata DownloadProfileMetadata `json:"metadata,omitempty"`
}

// DownloadProfilesResponse represent a list of profiles meant to be downloaded by user.
type DownloadProfilesResponse struct {
	Profiles []DownloadProfileItem `json:"profiles"`
}
