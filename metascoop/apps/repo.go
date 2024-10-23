package apps

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Application struct {
	AntiFeatures []string `yaml:"anti_features"`
	Categories   []string `yaml:"categories"`
	Description  string   `yaml:"description"`
	Filename     string   `yaml:"filename"`
	Id           string   `yaml:"id"`
	Name         string   `yaml:"name"`

	ReleaseDescription string
}
type Repo struct {
	GitURL       string        `yaml:"git"`
	Summary      string        `yaml:"summary"`
	Applications []Application `yaml:"applications"`

	Owner   string
	Name    string
	Host    string
	License string
}

func ParseRepoFile(filepath string) (list []Repo, err error) {
	f, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer f.Close()

	var repos map[string]Repo
	err = yaml.NewDecoder(f).Decode(&repos)
	if err != nil {
		return
	}

	for k, r := range repos {
		u, uerr := url.ParseRequestURI(r.GitURL)
		if uerr != nil {
			err = fmt.Errorf("problem with given git URL %q for repo with key=%q, name=%q: %w", r.GitURL, k, r.Name, uerr)
			return
		}
		split := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(split) < 2 {
			return
		}

		r.Owner = split[0]
		r.Name = split[1]
		r.Host = strings.TrimPrefix(u.Host, "www.")

		list = append(list, r)
	}

	return
}
