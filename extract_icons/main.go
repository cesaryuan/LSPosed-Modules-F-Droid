package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func extractApk(apkPath, tempDir string) error {
	// fmt.Printf("apktool d %s -o %s -f\n", apkPath, tempDir)
	cmd := exec.Command("apktool", "d", apkPath, "-o", tempDir, "-f")
	return cmd.Run()
}

func getManifestInfo(manifestPath string) (string, error) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}

	iconRegex := regexp.MustCompile(`android:icon="([^"]+)"`)
	iconMatch := iconRegex.FindStringSubmatch(string(content))
	if len(iconMatch) < 2 {
		return "", fmt.Errorf("无法在 AndroidManifest.xml 中找到有效的 icon 路径")
	}

	iconPath := strings.TrimPrefix(iconMatch[1], "@")

	return iconPath, nil
}

func mergeBackgroundSvgAndForegroundSvg(foregroundSvgPath, backgroundSvgPath string) error {
	// merge backgroundSvgPath to foregroundSvgPath
	foregroundSvgContent, err := os.ReadFile(foregroundSvgPath)
	if err != nil {
		return err
	}
	backgroundSvgContent, err := os.ReadFile(backgroundSvgPath)
	if err != nil {
		return err
	}
	foregroundMainContentRegex := regexp.MustCompile(`<svg[^>]*>(.*?</svg>)`)
	foregroundMainContentMatch := foregroundMainContentRegex.FindStringSubmatch(string(foregroundSvgContent))
	if len(foregroundMainContentMatch) > 1 {
		backgroundSvgContent = []byte(strings.Replace(string(backgroundSvgContent), `</svg>`, foregroundMainContentMatch[1], 1))
	}
	return os.WriteFile(foregroundSvgPath, backgroundSvgContent, 0644)
}

func convertColorFormat(color string) string {
	regex := regexp.MustCompile(`#([0-9A-Fa-f]{2})([0-9A-Fa-f]{6})`)
	if regex.MatchString(color) {
		return regex.ReplaceAllString(color, "#$2$1")
	}
	return color
}

func extractColorName(colorXmlPath, colorName string) (string, error) {
	knownColors := map[string]string{
		"@color/white":                      "#ffffffff",
		"@color/black":                      "#00000000",
		"@android:color/white":              "#ffffffff",
		"@android:color/black":              "#00000000",
		"@android:color/system_accent2_50":  "#f0f0f0",
		"@android:color/system_accent2_100": "#dddddd",
		"@android:color/system_accent2_600": "#999999",
		"@android:color/system_accent1_600": "#999999",
	}
	if color, ok := knownColors[colorName]; ok {
		return color, nil
	}
	if strings.HasPrefix(colorName, "@android:color/") {
		colorName = strings.TrimPrefix(colorName, "@android:color/")
		return colorName, nil
	}
	if strings.HasPrefix(colorName, "@color/") {
		colorXmlContent, err := os.ReadFile(colorXmlPath)
		if err != nil {
			return "", err
		}
		colorBase := filepath.Base(colorName)
		colorRegex := regexp.MustCompile(fmt.Sprintf(`<color name="%s">([^<]+)</color>`, colorBase))
		colorMatch := colorRegex.FindStringSubmatch(string(colorXmlContent))
		if len(colorMatch) > 1 {
			// format #AARRGGBB to #RRGGBBAA
			return convertColorFormat(colorMatch[1]), nil
		}
	}
	return "", fmt.Errorf("unknown color: %s", colorName)
}

func addOrChangeBackground(svgPath, backgroundColor string) error {
	svgContent, err := os.ReadFile(svgPath)
	if err != nil {
		return err
	}
	styleRegex := regexp.MustCompile(`(<svg[^>]* style="[^"]*background-color:)([^ ]+)`)
	if styleRegex.Match(svgContent) {
		svgContent = styleRegex.ReplaceAll(svgContent, []byte(fmt.Sprintf(`$1%s`, backgroundColor)))
	} else {
		svgContent = []byte(strings.Replace(string(svgContent), "<svg", `<svg style="background-color:`+backgroundColor+`"`, 1))
	}
	return os.WriteFile(svgPath, svgContent, 0644)
}

func findIconFile(resDir, iconPath string) (string, error) {
	// get prefix from iconName
	colorXmlPath := filepath.Join(resDir, "values", "colors.xml")
	prefix := strings.TrimPrefix(filepath.Dir(iconPath), "@")
	iconName := filepath.Base(iconPath)

	// find image format first
	extensions := []string{"webp", "png", "jpeg", "jpg", "svg"}
	for _, ext := range extensions {
		pattern := filepath.Join(resDir, prefix+"*", iconName+"."+ext)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
		if len(matches) > 0 {
			// order by size descending
			sort.Slice(matches, func(i, j int) bool {
				infoI, err := os.Stat(matches[i])
				if err != nil {
					return false
				}
				infoJ, err := os.Stat(matches[j])
				if err != nil {
					return false
				}
				return infoI.Size() > infoJ.Size()
			})
			return matches[0], nil
		}
	}

	// if no image format found, find xml file
	pattern := filepath.Join(resDir, prefix+"*", iconName+".xml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) > 0 {
		xmlContent, err := os.ReadFile(matches[0])
		if err != nil {
			return "", err
		}
		// 如果 xmlContent 是 <adaptive-icon>，提取 <foreground android:drawable="xxx" /> 中的 xxx
		if strings.Contains(string(xmlContent), "<adaptive-icon") {
			foregroundRegex := regexp.MustCompile(`<foreground android:drawable="([^"]+)"`)
			foregroundMatch := foregroundRegex.FindStringSubmatch(string(xmlContent))
			if len(foregroundMatch) > 1 {
				foregroundSvgPath, err := findIconFile(resDir, foregroundMatch[1])
				if err != nil {
					return "", err
				}
				if filepath.Ext(foregroundSvgPath) == ".svg" {
					backgroundRegex := regexp.MustCompile(`<background android:drawable="([^"]+)"`)
					backgroundMatch := backgroundRegex.FindStringSubmatch(string(xmlContent))
					if len(backgroundMatch) > 1 {
						backgroundColor, err := extractColorName(colorXmlPath, backgroundMatch[1])
						if err == nil {
							if err := addOrChangeBackground(foregroundSvgPath, backgroundColor); err != nil {
								fmt.Errorf("Failed to add or change background: %v\n", err)
							}
						} else {
							fmt.Printf("Failed to extract background color name: %v\n", err)
							backgroundSvgPath, err := findIconFile(resDir, backgroundMatch[1])
							if err != nil {
								fmt.Printf("Failed to find background SVG: %v\n", err)
							} else {
								if err := mergeBackgroundSvgAndForegroundSvg(foregroundSvgPath, backgroundSvgPath); err != nil {
									fmt.Printf("Failed to merge background and foreground SVG: %v\n", err)
								}
							}
						}
					}
				}
				return foregroundSvgPath, nil
			}
		}
		// 如果是 <vector>，转换为 svg
		if strings.Contains(string(xmlContent), "<vector") {
			svgPath := filepath.Join(resDir, iconName+".svg")
			cmd := exec.Command("npx", "vector-drawable-svg", matches[0], svgPath)
			if err := cmd.Run(); err != nil {
				// 调用 openai 转换为 svg
				svg, err := converVectorToSvg(string(xmlContent))
				if err != nil {
					return "", err
				}
				// 保存 svg

				svg = strings.TrimSpace(svg)
				svg = strings.TrimSuffix(svg, "```")
				svg = strings.TrimLeftFunc(svg, func(r rune) bool {
					return r != '<'
				})
				if err := os.WriteFile(svgPath, []byte(svg), 0644); err != nil {
					return "", err
				}
				return svgPath, nil
			}
			// 处理 svg 中形如 @android:color/xxx, @color/xxx 的 color
			colorRegex := regexp.MustCompile(`@android:color/[^,"]+|@color/[^,"]+`)
			svgContent, err := os.ReadFile(svgPath)
			if err != nil {
				return "", err
			}
			svgContent = colorRegex.ReplaceAllFunc(svgContent, func(match []byte) []byte {
				colorName := string(match)
				color, err := extractColorName(colorXmlPath, colorName)
				if err != nil {
					return match
				}
				return []byte(color)
			})

			// 去除 width="xxx.xd" height="xxx.xd" 中的 d
			svgContent = regexp.MustCompile(`((width|height)="[\d\.]+)d"`).ReplaceAll(svgContent, []byte("$1"))
			if err := os.WriteFile(svgPath, svgContent, 0644); err != nil {
				return "", err
			}
			return svgPath, nil
		}
	}

	return "", fmt.Errorf("无法找到匹配的图标文件")
}

func copyIconFile(iconFile, outputDir, packageName string) (string, error) {
	iconExt := filepath.Ext(iconFile)
	outputPath := filepath.Join(outputDir, packageName+iconExt)

	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return "", err
	}

	input, err := os.ReadFile(iconFile)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(outputPath, input, 0644); err != nil {
		return "", err
	}

	return outputPath, nil
}

func converVectorToSvg(xmlContent string) (string, error) {
	prompt := `
You are tasked with converting an Android vector drawable XML to an SVG (Scalable Vector Graphics) format. The input Android vector XML will be provided to you, and you need to transform it into a valid SVG format. Follow these steps carefully:

1. To convert this Android vector to SVG, follow these steps:

a. Remove all Android-specific attributes (those with the "android:" prefix).
b. Convert the <vector> tag to an <svg> tag.
c. Set the viewBox attribute of the <svg> tag using the values from android:viewportWidth and android:viewportHeight. The format should be viewBox="0 0 [width] [height]".
d. For transform, translate should be placed in the first place.
e. Any tag should be closed correctly.

d. For each <path> element:
   - Keep the "d" attribute (pathData in Android becomes d in SVG).
   - Convert android:fillColor to fill.
   - Convert android:strokeColor to stroke.
   - Convert android:strokeWidth to stroke-width.
   - Remove any Android-specific attributes.
   - For color, the android's format is #AARRGGBB, you need to convert it to #RRGGBBAA.

e. If there are any @drawable references (like "@drawable/$ic_launcher_foreground__0"), replace them with a placeholder color (e.g., "#00000000") and add a comment noting that this color needs to be manually replaced.

2. Output your converted SVG inside <svg> tags. Make sure to include the XML declaration and the SVG namespace.
3. If all elements's color are white, this means we should add a background color to the svg. For example color: #2B7CD3
4. Do not output any other redundant things, such as code block identifiers. And don't add anyexplanations, comments, and so on.

Remember, the goal is to create a valid SVG that closely resembles the original Android vector drawable. Some complex features might not have direct SVG equivalents, so use your best judgment to approximate them.
    `
	client := openai.NewClient(
		// defaults to os.LookupEnv("OPENAI_API_KEY")
		option.WithBaseURL("https://api.deepseek.com/v1"),
	)
	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
			openai.UserMessage(fmt.Sprintf("The following is the Android vector XML: %s", xmlContent)),
		}),
		Model: openai.F("deepseek-chat"),
	})
	if err != nil {
		return "", err
	}
	return chatCompletion.Choices[0].Message.Content, nil
}

func processApp(app map[string]interface{}, indexJson map[string]interface{}, mu *sync.Mutex) {
	packageName := app["packageName"].(string)
	pattern := filepath.Join(".", "fdroid", "repo", "icons", packageName+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Failed to get icon file: %v\n", err)
		return
	}
	if len(matches) > 0 {
		fmt.Printf("Icon file already exists: %s\n", matches[0])
		return
	}

	fmt.Printf("Processing app: %s\n", packageName)
	apkInfo, exists := indexJson["packages"].(map[string]interface{})[packageName]
	if !exists || len(apkInfo.([]interface{})) == 0 {
		fmt.Printf("No APK info found for %s\n", packageName)
		return
	}

	apkFileName := apkInfo.([]interface{})[0].(map[string]interface{})["apkName"].(string)
	apkPath := filepath.Join("fdroid", "repo", apkFileName)

	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		fmt.Printf("APK file not found: %s\n", apkPath)
		return
	}

	unpackDir := filepath.Join(os.TempDir(), "temp_apktool_unpack", packageName)
	if err := os.MkdirAll(unpackDir, os.ModePerm); err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		return
	}
	defer os.RemoveAll(unpackDir)

	if err := extractApk(apkPath, unpackDir); err != nil && err.Error() != "exit status 1" {
		fmt.Printf("Failed to extract APK: %v\n", err)
		return
	}

	manifestPath := filepath.Join(unpackDir, "AndroidManifest.xml")
	iconName, err := getManifestInfo(manifestPath)
	if err != nil {
		fmt.Printf("Failed to get manifest info: %v\n", err)
		return
	}

	resDir := filepath.Join(unpackDir, "res")
	iconFile, err := findIconFile(resDir, iconName)
	if err != nil {
		fmt.Printf("Failed to find icon file: %v\n", err)
		return
	}

	outputPath, err := copyIconFile(iconFile, "./fdroid/repo/icons", packageName)
	copyIconFile(iconFile, "./fdroid/repo/icons-120", packageName)
	copyIconFile(iconFile, "./fdroid/repo/icons-160", packageName)
	copyIconFile(iconFile, "./fdroid/repo/icons-240", packageName)
	copyIconFile(iconFile, "./fdroid/repo/icons-320", packageName)
	copyIconFile(iconFile, "./fdroid/repo/icons-480", packageName)
	copyIconFile(iconFile, "./fdroid/repo/icons-640", packageName)
	if err != nil {
		fmt.Printf("Failed to copy icon file: %v\n", err)
		return
	}

	fmt.Printf("Icon extracted successfully for %s: %s\n", app["packageName"], outputPath)

	mu.Lock()
	app["icon"] = filepath.Base(outputPath)
	mu.Unlock()
}
func main() {
	data, err := ioutil.ReadFile("fdroid/repo/index-v1.json")
	if err != nil {
		fmt.Printf("Failed to read index JSON: %v\n", err)
		return
	}

	var indexJson map[string]interface{}
	if err := json.Unmarshal(data, &indexJson); err != nil {
		fmt.Printf("Failed to parse index JSON: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 10)

	apps, ok := indexJson["apps"].([]interface{})
	if !ok {
		fmt.Printf("Failed to get apps\n")
		return
	}
	for _, app := range apps {
		appMap, ok := app.(map[string]interface{})
		if !ok {
			fmt.Printf("Failed to get app\n")
			continue
		}
		icon, ok := appMap["icon"]
		if ok {
			iconPath := filepath.Join("fdroid/repo/icons", icon.(string))
			if _, err := os.Stat(iconPath); !os.IsNotExist(err) {
				continue
			}
		}
		// pkgName := appMap["packageName"].(string)
		// if pkgName != "de.buttercookie.disabletargetapiblock" {
		// 	continue
		// }

		wg.Add(1)
		semaphore <- struct{}{}
		go func(app map[string]interface{}) {
			defer wg.Done()
			defer func() { <-semaphore }()
			processApp(appMap, indexJson, &mu)
		}(appMap)
	}

	wg.Wait()
	fmt.Println("Processing completed.")
}
