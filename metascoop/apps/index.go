package apps

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/r3labs/diff/v3"
)

type RepoIndex struct {
	Repo     map[string]interface{}   `json:"repo"`
	Requests map[string]interface{}   `json:"requests"`
	Apps     []map[string]interface{} `json:"apps"`

	Packages map[string][]PackageInfo `json:"packages"`
}

type PackageInfo struct {
	Added            int64    `json:"added"`
	ApkName          string   `json:"apkName"`
	Hash             string   `json:"hash"`
	HashType         string   `json:"hashType"`
	MinSdkVersion    int      `json:"minSdkVersion"`
	Nativecode       []string `json:"nativecode"`
	PackageName      string   `json:"packageName"`
	Sig              string   `json:"sig"`
	Signer           string   `json:"signer"`
	Size             int      `json:"size"`
	TargetSdkVersion int      `json:"targetSdkVersion"`
	VersionCode      int      `json:"versionCode,omitempty"`
	VersionName      string   `json:"versionName"`
}

func (r *RepoIndex) FindLatestPackage(pkgName string) (p PackageInfo, ok bool) {
	pkgs, ok := r.Packages[pkgName]
	if !ok {
		return p, false
	}

	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].VersionCode != pkgs[j].VersionCode {
			return pkgs[i].VersionCode < pkgs[j].VersionCode
		}

		v1, err := version.NewVersion(pkgs[i].VersionName)
		if err != nil {
			return true
		}

		v2, err := version.NewVersion(pkgs[i].VersionName)
		if err != nil {
			return false
		}

		return v1.LessThan(v2)
	})

	// Return the one with the latest version
	return pkgs[len(pkgs)-1], true
}

func ReadIndex(path string) (index *RepoIndex, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&index)

	return
}

// HasSignificantChanges compares two RepoIndex structs and determines if there are significant changes between them.
//
// Parameters:
//   - old: A pointer to the original RepoIndex struct.
//   - new: A pointer to the updated RepoIndex struct.
//
// Returns:
//   - changedPath: A string representing the JSON path where a significant change was found.
//   - changed: A boolean indicating whether a significant change was detected (true) or not (false).
//
// The function uses the diff package to compare the two structs. It ignores certain changes that are considered
// non-significant, such as updates to the "added" or "lastUpdated" timestamps of apps, and updates to the repo timestamp.
// Any other changes are considered significant.
//
// If a significant change is found, the function returns the path to the change and true.
// If no significant changes are found, it returns an empty string and false.

func HasSignificantChanges(old, new *RepoIndex) (changedPath string, changed bool) {
	changelog, err := diff.Diff(old, new)
	if err != nil {
		panic("diffing fdroid index structs: " + err.Error())
	}

	for _, change := range changelog {
		if change.Type != diff.UPDATE {
			return strings.Join(change.Path, "."), true
		}

		var isIgnoredChange = false

		// Fdroid seems to update the "added" timestamp of apps every time we run the command
		if len(change.Path) > 0 && (strings.EqualFold(change.Path[len(change.Path)-1], "added") || strings.EqualFold(change.Path[len(change.Path)-1], "lastUpdated")) {
			isIgnoredChange = true
		}

		// Also it updates the repo timestamp
		if reflect.DeepEqual(change.Path, []string{"Repo", "timestamp"}) {
			isIgnoredChange = true
		}

		if !isIgnoredChange {
			return strings.Join(change.Path, "."), true
		}
	}

	return "", false
}
