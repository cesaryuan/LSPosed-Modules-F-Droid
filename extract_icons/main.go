package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type App struct {
	PackageName string `json:"packageName"`
	Icon        string `json:"icon,omitempty"`
}

type IndexJson struct {
	Apps     []App `json:"apps"`
	Packages map[string][]struct {
		ApkName string `json:"apkName"`
	} `json:"packages"`
}

func extractApk(apkPath, tempDir string) error {
	fmt.Printf("apktool d %s -o %s -f\n", apkPath, tempDir)
	cmd := exec.Command("apktool", "d", apkPath, "-o", tempDir, "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getManifestInfo(manifestPath string) (string, string, error) {
	content, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return "", "", err
	}

	iconRegex := regexp.MustCompile(`android:icon="([^"]+)"`)
	packageRegex := regexp.MustCompile(`package="([^"]+)"`)

	iconMatch := iconRegex.FindStringSubmatch(string(content))
	packageMatch := packageRegex.FindStringSubmatch(string(content))

	if len(iconMatch) < 2 || len(packageMatch) < 2 {
		return "", "", fmt.Errorf("无法在 AndroidManifest.xml 中找到有效的 icon 路径或包名")
	}

	iconPath := strings.TrimPrefix(iconMatch[1], "@")
	packageName := packageMatch[1]

	return iconPath, packageName, nil
}

func findIconFile(resDir, iconName string) (string, error) {
	mipmapDirs, err := ioutil.ReadDir(resDir)
	if err != nil {
		return "", err
	}
	// get prefix from iconName
	prefix := strings.Split(iconName, "/")[0]
	iconName = strings.Split(iconName, "/")[1]
	prefix = strings.TrimPrefix(prefix, "@")
	extensions := []string{"webp", "png", "jpeg", "jpg", "svg", "xml"}

	for _, ext := range extensions {
		for _, dir := range mipmapDirs {
			if strings.HasPrefix(dir.Name(), prefix) {
				potentialIcon := filepath.Join(resDir, dir.Name(), iconName+"."+ext)
				if _, err := os.Stat(potentialIcon); err == nil {
					return potentialIcon, nil
				}
			}
		}
	}

	return "", fmt.Errorf("无法找到匹配的图标文件")
}

func copyIconFile(iconFile, outputDir, packageName string) (string, error) {
	iconExt := filepath.Ext(iconFile)
	outputPath := filepath.Join(outputDir, "fdroid", "repo", "icons", packageName+iconExt)

	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return "", err
	}

	input, err := ioutil.ReadFile(iconFile)
	if err != nil {
		return "", err
	}

	if err := ioutil.WriteFile(outputPath, input, 0644); err != nil {
		return "", err
	}

	return outputPath, nil
}

func processApp(app App, indexJson *IndexJson, mu *sync.Mutex) {

	fmt.Printf("Processing app: %s\n", app.PackageName)

	apkInfo, exists := indexJson.Packages[app.PackageName]
	if !exists || len(apkInfo) == 0 {
		fmt.Printf("No APK info found for %s\n", app.PackageName)
		return
	}

	apkFileName := apkInfo[0].ApkName
	apkPath := filepath.Join("fdroid", "repo", apkFileName)

	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		fmt.Printf("APK file not found: %s\n", apkPath)
		return
	}

	tempDir, err := ioutil.TempDir("", "apk_extract_")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	if err := extractApk(apkPath, tempDir); err != nil && err.Error() != "exit status 1" {
		fmt.Printf("Failed to extract APK: %v\n", err)
		return
	}

	manifestPath := filepath.Join(tempDir, "AndroidManifest.xml")
	iconName, packageName, err := getManifestInfo(manifestPath)
	if err != nil {
		fmt.Printf("Failed to get manifest info: %v\n", err)
		return
	}

	resDir := filepath.Join(tempDir, "res")
	iconFile, err := findIconFile(resDir, iconName)
	if err != nil {
		fmt.Printf("Failed to find icon file: %v\n", err)
		return
	}

	outputPath, err := copyIconFile(iconFile, ".", packageName)
	if err != nil {
		fmt.Printf("Failed to copy icon file: %v\n", err)
		return
	}

	fmt.Printf("Icon extracted successfully for %s: %s\n", app.PackageName, outputPath)

	mu.Lock()
	app.Icon = filepath.Base(outputPath)
	mu.Unlock()
}
func main() {
	data, err := ioutil.ReadFile("fdroid/repo/index-v1.json")
	if err != nil {
		fmt.Printf("Failed to read index JSON: %v\n", err)
		return
	}

	var indexJson IndexJson
	if err := json.Unmarshal(data, &indexJson); err != nil {
		fmt.Printf("Failed to parse index JSON: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 4) // 限制同时运行的线程数为 5

	for _, app := range indexJson.Apps {
		if app.Icon == "" {
			wg.Add(1)
			semaphore <- struct{}{} // 获取信号量
			go func(app App) {
				defer wg.Done()
				defer func() { <-semaphore }() // 释放信号量
				processApp(app, &indexJson, &mu)
			}(app)
		}
	}

	wg.Wait()

	updatedData, err := json.MarshalIndent(indexJson, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal updated JSON: %v\n", err)
		return
	}

	if err := ioutil.WriteFile("fdroid/repo/index-v1.json", updatedData, 0644); err != nil {
		fmt.Printf("Failed to write updated JSON: %v\n", err)
		return
	}

	fmt.Println("Processing completed.")
}
