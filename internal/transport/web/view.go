// Package web は HTTP ハンドラとビュー (html/template) を提供する。
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

//go:embed all:templates all:static
var assets embed.FS

// Templates は読み込み済みの html/template 群。
type Templates struct {
	pages map[string]*template.Template
}

// LoadTemplates は embed.FS からテンプレートを読み込む。
func LoadTemplates() (*Templates, error) {
	layoutBytes, err := fs.ReadFile(assets, "templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("read layout: %w", err)
	}
	pages := map[string]*template.Template{}
	funcs := template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
		"safehtml": func(s string) template.HTML {
			return template.HTML(s) //nolint:gosec // SVG はサーバ生成で固定文言、ユーザテキストは別途エスケープ済み
		},
	}
	names := []string{"home", "login", "register", "quiz", "answer", "progress", "history", "settings"}
	for _, n := range names {
		page, err := fs.ReadFile(assets, "templates/"+n+".html")
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", n, err)
		}
		t := template.New(n).Funcs(funcs)
		if _, err := t.Parse(string(layoutBytes)); err != nil {
			return nil, fmt.Errorf("parse layout for %s: %w", n, err)
		}
		if _, err := t.Parse(string(page)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", n, err)
		}
		pages[n] = t
	}
	return &Templates{pages: pages}, nil
}

// Render は指定テンプレートをレンダリングする。
func (t *Templates) Render(w io.Writer, name string, data any) error {
	tpl, ok := t.pages[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	return tpl.ExecuteTemplate(w, "layout", data)
}

// StaticFS は /static/* の配信用 fs.FS を返す。
func StaticFS() (fs.FS, error) {
	return fs.Sub(assets, "static")
}
