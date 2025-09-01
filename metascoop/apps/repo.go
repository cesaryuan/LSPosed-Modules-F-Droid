package apps

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"sigs.k8s.io/yaml"
)

type Application struct {
	AntiFeatures []string `yaml:"anti_features"`
	Categories   []string `yaml:"categories"`
	Description  string   `yaml:"description"`
	Filename     string   `yaml:"filename"`
	Id           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	LastUpdated  string   `yaml:"last_updated"`

	ReleaseDescription string
}
type Repo struct {
	GitURL       string        `yaml:"git"`
	Summary      string        `yaml:"summary"`
	Applications []Application `yaml:"applications"`
	AuthorName   string        `yaml:"author_name"`

	Owner   string `yaml:"owner,omitempty"`
	Name    string `yaml:"name,omitempty"`
	Host    string `yaml:"host,omitempty"`
	License string `yaml:"license,omitempty"`
}

func ParseRepoFile(filepath string) (list []Repo, err error) {
	f, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := os.ReadFile(filepath)
	if err != nil {
		return
	}

	var repos map[string]Repo
	err = yaml.Unmarshal(data, &repos)
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
		if strings.TrimSpace(r.AuthorName) == "" {
			r.AuthorName = r.Owner
		}

		list = append(list, r)
	}

	return
}
