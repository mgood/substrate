package prompts

import (
	"embed"
	"io"
	"strings"
	"text/template"

	"tractor.dev/toolkit-go/engine/fs"
)

//go:embed *
var dirEmbed embed.FS

var dir = fs.LiveDir(dirEmbed)

func baseTemplate() *template.Template {
	return template.New("").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	})
}

func loadTemplates() (*template.Template, error) {
	// TODO if we're reading from the embed.FS the contents won't change, so we
	// should cache this forever
	return baseTemplate().ParseFS(dir, "*.tmpl")
}

func Execute(w io.Writer, name string, data any) error {
	tmpl, err := loadTemplates()
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, name+".tmpl", data)
}

func Render(name string, data any) (string, error) {
	var w strings.Builder
	err := Execute(&w, name, data)
	return w.String(), err
}

func RenderToString(t *template.Template, data any) (string, error) {
	var w strings.Builder
	err := t.Execute(&w, data)
	return w.String(), err
}

func ParseTemplate(tmpl string) (*template.Template, error) {
	return baseTemplate().Parse(tmpl)
}