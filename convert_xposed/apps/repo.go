package apps

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
