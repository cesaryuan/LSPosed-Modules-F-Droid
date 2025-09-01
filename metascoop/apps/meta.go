package apps

import (
	"os"

	"sigs.k8s.io/yaml"
)

func ReadMetaFile(path string) (d map[string]interface{}, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(data, &d)

	return
}

func WriteMetaFile(path string, data map[string]interface{}) (err error) {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return
	}

	mdata, err := yaml.Marshal(data)
	if err != nil {
		return
	}

	err = os.WriteFile(tmpPath, mdata, 0644)
	if err != nil {
		return
	}

	if err != nil {
		_ = f.Close()
		return
	}

	err = f.Close()
	if err != nil {
		return
	}

	return os.Rename(tmpPath, path)
}
