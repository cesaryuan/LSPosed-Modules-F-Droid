package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"metascoop/apps"

	"gopkg.in/yaml.v3"
)

type JSONItem struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	URL           string `json:"url"`
	Collaborators []struct {
		Login string `json:"login"`
	} `json:"collaborators"`
	Summary  string `json:"summary"`
	Releases []struct {
		ReleaseAssets []struct {
			Name string `json:"name"`
		} `json:"releaseAssets"`
		DescriptionHTML string `json:"descriptionHTML"`
	} `json:"releases"`
}

func getJSONData() ([]byte, error) {
	resp, err := http.Get("https://modules.lsposed.org/modules.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func main() {
	// Parse command line arguments
	outputPath := flag.String("o", "", "Output YAML file path")
	maxItems := flag.Int("max", 0, "Maximum number of items to convert")
	flag.Parse()
	if *outputPath == "" {
		log.Fatal("Error: Output path (-o) is required")
	}

	// get json data from https://modules.lsposed.org/modules.json
	jsonData, err := getJSONData()
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}

	var jsonItems []JSONItem
	err = json.Unmarshal(jsonData, &jsonItems)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %v", err)
	}

	repos := make(map[string]apps.Repo)

	if *maxItems == 0 {
		*maxItems = len(jsonItems)
	}
	fmt.Println("Length of jsonItems to convert:", *maxItems)

	for _, item := range jsonItems[:min(len(jsonItems), *maxItems)] {
		if len(item.Releases) > 1 {
			log.Fatalf("Error: More than one release for item %s", item.Name)
		}

		repo := apps.Repo{
			GitURL:  item.URL,
			Summary: item.Summary,
			Applications: []apps.Application{
				{
					Filename:           item.Releases[0].ReleaseAssets[0].Name,
					Id:                 item.Name,
					Name:               item.Description,
					Categories:         []string{"Xposed"},
					Description:        item.Releases[0].DescriptionHTML,
					ReleaseDescription: item.Releases[0].DescriptionHTML,
				},
			},
		}

		if len(item.Collaborators) > 0 {
			repo.Owner = item.Collaborators[0].Login
		}

		repos[item.Name] = repo
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(repos)
	if err != nil {
		log.Fatalf("Error marshaling to YAML: %v", err)
	}

	// Write YAML file
	err = ioutil.WriteFile(*outputPath, yamlData, 0644)
	if err != nil {
		log.Fatalf("Error writing YAML file: %v", err)
	}

	fmt.Printf("Conversion completed. Output written to %s\n", *outputPath)
}
