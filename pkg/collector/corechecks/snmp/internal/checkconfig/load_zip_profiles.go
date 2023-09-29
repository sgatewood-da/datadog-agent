package checkconfig

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/networkdevice/profile/profiledefinition"
	"io"
)

func loadZipProfiles() (profileConfigMap, error) {
	zipFilePath := getZipFilePath()
	reader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		fmt.Printf("file.Name: %s\n", file.Name)
		if file.Name == "custom.profiles" {
			open, err := file.Open()
			if err != nil {
				return nil, err
			}
			all, err := io.ReadAll(open)
			if err != nil {
				return nil, err
			}
			fmt.Printf("CONTENT: %s\n", string(all))
			downloadedProfiles := profiledefinition.DownloadProfilesResponse{}
			err = json.Unmarshal(all, &downloadedProfiles)
			if err != nil {
				return nil, err
			}
			//fmt.Printf("downloadedProfiles: %#v\n", downloadedProfiles)
			//fmt.Printf("downloadedProfiles: %#v\n", downloadedProfiles)

			profiles := make(profileConfigMap)
			for _, profile := range downloadedProfiles.Profiles {
				if profile.Profile.Name == "" {
					//return nil, fmt.Errorf("a profile from zip have a missing name")
					// TODO: raise error?
					continue
				}
				profiles[profile.Profile.Name] = profileConfig{Definition: profile.Profile}
			}
			return profiles, nil
		}
	}
	return nil, fmt.Errorf("HANDLE ZIP FILE")
}

func getZipFilePath() string {
	return getProfileConfdRoot(profilesZipFile)
}
