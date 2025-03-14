package md

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"strings"

	"metascoop/apps"
)

const (
	tableStart = "<!-- This table is auto-generated. Do not edit -->"

	tableEnd = "<!-- end apps table -->"

	tableTmpl = `
| Icon | Name | Description | Version |
| --- | --- | --- | --- |{{range .Apps}}
| <a href="{{.sourceCode}}"><img src="{{if .icon}}fdroid/repo/icons/{{.icon}}{{else}}assets/ic_launcher.png{{end}}" alt="{{.name | handleCRandBR}} icon" width="36px" height="36px"></a> | [**{{.name | handleCRandBR}}**]({{.sourceCode}}) | {{if .summary}}{{.summary | handleCRandBR}}{{else}}No summary available{{end}} | {{.suggestedVersionName}} ({{.suggestedVersionCode}}) |{{end}}
` + tableEnd
)

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	"replace": strings.ReplaceAll,
	"handleCRandBR": func(s string) template.HTML {
		// replace \r\n with <br>
		result := strings.ReplaceAll(s, "\r\n", "<br>")
		result = strings.ReplaceAll(result, "\n", "<br>")
		result = strings.ReplaceAll(result, "|", "\\|")
		return template.HTML(result)
	},
}).Parse(tableTmpl))

func RegenerateReadme(readMePath string, index *apps.RepoIndex, repoURL string) (err error) {
	content, err := os.ReadFile(readMePath)
	if err != nil {
		return
	}

	var tableStartIndex = bytes.Index(content, []byte(tableStart))
	if tableStartIndex < 0 {
		return fmt.Errorf("cannot find table start in %q", readMePath)
	}

	var tableEndIndex = bytes.Index(content, []byte(tableEnd))
	if tableEndIndex < 0 {
		return fmt.Errorf("cannot find table end in %q", readMePath)
	}

	var table bytes.Buffer

	table.WriteString(tableStart)

	err = tmpl.Execute(&table, index)
	if err != nil {
		return err
	}

	newContent := []byte{}

	newContent = append(newContent, content[:tableStartIndex]...)
	newContent = append(newContent, table.Bytes()...)
	newContent = append(newContent, content[tableEndIndex:]...)

	// Replace the repo URL in the new content
	// https://[\w\.\/\-]+\?fingerprint=[\w]+
	re := regexp.MustCompile(`https://[\w\.\/\-]+\?fingerprint=[\w]+`)
	newContent = re.ReplaceAll(newContent, []byte(repoURL))

	re_intent := regexp.MustCompile(`froidrepos://[\w\.\/\-]+\?fingerprint=[\w]+`)
	newContent = re_intent.ReplaceAll(newContent, []byte(strings.Replace(repoURL, "https://", "froidrepos://", 1)))

	return os.WriteFile(readMePath, newContent, os.ModePerm)
}
