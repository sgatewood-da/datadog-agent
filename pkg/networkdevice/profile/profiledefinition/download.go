package profiledefinition

// DownloadProfileItem represent a profile.
type DownloadProfileItem struct {
	Profile ProfileDefinition `json:"profile"`
}

// DownloadProfilesResponse represent a list of profiles meant to be downloaded by user.
type DownloadProfilesResponse struct {
	DownloadedAt string                `json:"downloaded_at"`
	Profiles     []DownloadProfileItem `json:"profiles"`
}
