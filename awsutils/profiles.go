package awsutils

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"gopkg.in/ini.v1"
)

func GetAwsProfiles() ([]string, error) {
	fname := config.DefaultSharedConfigFilename()
	f, err := ini.Load(fname)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config file: %v", err)
	}

	var profiles []string
	for _, section := range f.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			profiles = append(profiles, "default")
		} else if len(section.Keys()) > 0 {
			const prefix = "profile "
			if len(name) > len(prefix) && name[:len(prefix)] == prefix {
				profiles = append(profiles, name[len(prefix):])
			} else {
				profiles = append(profiles, name)
			}
		}
	}
	return profiles, nil
}
