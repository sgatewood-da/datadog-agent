package checkconfig

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/networkdevice/profile/profiledefinition"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"io"
	"os"
	"path/filepath"
)

func loadZipProfiles() (profileConfigMap, error) {
	zipFilePath := getGZipFilePath()
	file, err := os.Open(zipFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	all, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	fmt.Printf("CONTENT: %s\n", string(all))
	downloadedProfiles := profiledefinition.ProfileBundleResponse{}
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

		if _, exist := profiles[profile.Profile.Name]; exist {
			// TODO: this should not happen
			log.Warnf("duplicate profile found: %s", profile.Profile.Name)
			continue
		}
		profiles[profile.Profile.Name] = profileConfig{Definition: profile.Profile}
	}
	return profiles, nil
}

func getGZipFilePath() string {
	return getProfileConfdRoot(filepath.Join(userProfilesFolder, profilesZipFile))
}
