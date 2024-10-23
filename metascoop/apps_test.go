package main

import (
	"fmt"
	"strings"
	"testing"

	"metascoop/apps"
)

func TestRepoFile(t *testing.T) {
	reposList, err := apps.ParseRepoFile("../repos.yaml")
	if err != nil {
		t.Errorf("error parsing repos file: %s", err.Error())
	}
	if len(reposList) == 0 {
		t.Errorf("the repo list is empty, wanted at least one repo")
	}
	if len(reposList[0].Applications) == 0 {
		t.Errorf("the repo app list is empty, wanted at least one app")
	}

	for _, r := range reposList {
		if strings.Contains(r.Name, "Bitwarden") == false {
			t.Errorf("the repo name does not contain bitwarden. name=%s", r.Name)
		}
		fmt.Printf("repo=%q\n", r)

		if len(r.Applications) == 0 {
			t.Errorf("the repo contains no applications. at least one application is expected")
		}
		for _, a := range r.Applications {
			fmt.Printf("app=%q\n", a)
		}
	}
}
