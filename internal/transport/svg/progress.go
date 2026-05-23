// Package svg は進捗グラフを SVG として生成する。
//
// 外部依存は使わず、html/template 経由でなく直接文字列を組み立てる。
// ユーザ入力は数値 (int/float64) のみで、テキスト出力は固定文言のため XSS リスクなし。
package svg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

// DailyAccuracyChart は日次正解率 (0..100%) を折れ線で描く SVG を返す。
//
// width/height は viewBox 単位。色 #b80f0a (朱) を主線に、薄い灰の grid を背景に置く。
func DailyAccuracyChart(data []domain.DailyAccuracy, width, height int) string {
	if width <= 0 {
		width = 720
	}
	if height <= 0 {
		height = 240
	}
	padL, padR, padT, padB := 40, 12, 16, 28
	plotW := width - padL - padR
	plotH := height - padT - padB

	var sb strings.Builder
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	sb.WriteString(strconv.Itoa(width))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(height))
	sb.WriteString(`" role="img" aria-label="日次正解率グラフ">`)
	// 背景 grid
	for i := 0; i <= 4; i++ {
		y := padT + plotH*i/4
		sb.WriteString(`<line x1="`)
		sb.WriteString(strconv.Itoa(padL))
		sb.WriteString(`" y1="`)
		sb.WriteString(strconv.Itoa(y))
		sb.WriteString(`" x2="`)
		sb.WriteString(strconv.Itoa(padL + plotW))
		sb.WriteString(`" y2="`)
		sb.WriteString(strconv.Itoa(y))
		sb.WriteString(`" stroke="#e6e1d6" stroke-width="1"/>`)
		// y label
		sb.WriteString(`<text x="`)
		sb.WriteString(strconv.Itoa(padL - 6))
		sb.WriteString(`" y="`)
		sb.WriteString(strconv.Itoa(y + 4))
		sb.WriteString(`" text-anchor="end" font-size="10" fill="#3a3a3a">`)
		sb.WriteString(strconv.Itoa(100 - 25*i))
		sb.WriteString(`%</text>`)
	}
	if len(data) == 0 {
		sb.WriteString(`<text x="`)
		sb.WriteString(strconv.Itoa(padL + plotW/2))
		sb.WriteString(`" y="`)
		sb.WriteString(strconv.Itoa(padT + plotH/2))
		sb.WriteString(`" text-anchor="middle" font-size="14" fill="#7a7a7a">データなし</text>`)
		sb.WriteString(`</svg>`)
		return sb.String()
	}

	// polyline points
	n := len(data)
	var pts strings.Builder
	for i, d := range data {
		var rate float64
		if d.Total > 0 {
			rate = float64(d.Correct) / float64(d.Total)
		}
		x := padL
		if n > 1 {
			x = padL + plotW*i/(n-1)
		}
		y := padT + plotH - int(rate*float64(plotH))
		if i > 0 {
			pts.WriteByte(' ')
		}
		pts.WriteString(strconv.Itoa(x))
		pts.WriteByte(',')
		pts.WriteString(strconv.Itoa(y))
	}
	sb.WriteString(`<polyline fill="none" stroke="#b80f0a" stroke-width="2" points="`)
	sb.WriteString(pts.String())
	sb.WriteString(`"/>`)
	// 点
	for i, d := range data {
		var rate float64
		if d.Total > 0 {
			rate = float64(d.Correct) / float64(d.Total)
		}
		x := padL
		if n > 1 {
			x = padL + plotW*i/(n-1)
		}
		y := padT + plotH - int(rate*float64(plotH))
		sb.WriteString(`<circle cx="`)
		sb.WriteString(strconv.Itoa(x))
		sb.WriteString(`" cy="`)
		sb.WriteString(strconv.Itoa(y))
		sb.WriteString(`" r="3" fill="#b80f0a"/>`)
	}
	// x labels (端と中央)
	labels := []int{0, n - 1}
	if n >= 3 {
		labels = append(labels, n/2)
	}
	for _, i := range labels {
		x := padL + plotW*i/(n-1)
		if n == 1 {
			x = padL + plotW/2
		}
		sb.WriteString(`<text x="`)
		sb.WriteString(strconv.Itoa(x))
		sb.WriteString(`" y="`)
		sb.WriteString(strconv.Itoa(padT + plotH + 16))
		sb.WriteString(`" text-anchor="middle" font-size="10" fill="#3a3a3a">`)
		sb.WriteString(data[i].Date.Format("01/02"))
		sb.WriteString(`</text>`)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

// TopicAccuracyBars は論点別正解率を水平バーで描く SVG を返す。
func TopicAccuracyBars(stats []domain.TopicStat, width int) string {
	if width <= 0 {
		width = 720
	}
	rowH := 26
	padL, padR, padT, padB := 140, 20, 12, 12
	height := padT + padB + rowH*max(1, len(stats))
	plotW := width - padL - padR

	var sb strings.Builder
	fmt.Fprintf(&sb,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" role="img" aria-label="論点別正解率">`,
		width, height)
	if len(stats) == 0 {
		fmt.Fprintf(&sb,
			`<text x="%d" y="%d" text-anchor="middle" font-size="14" fill="#7a7a7a">データなし</text></svg>`,
			width/2, height/2)
		return sb.String()
	}
	for i, s := range stats {
		y := padT + rowH*i + rowH/2
		rate := s.Accuracy()
		barW := int(float64(plotW) * rate)
		// label
		fmt.Fprintf(&sb,
			`<text x="%d" y="%d" text-anchor="end" font-size="12" fill="#3a3a3a">%s</text>`,
			padL-6, y+4, htmlEscape(s.TopicName))
		// 背景
		fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="14" fill="#efe9dc"/>`, padL, y-7, plotW)
		// 値 bar
		fmt.Fprintf(&sb, `<rect x="%d" y="%d" width="%d" height="14" fill="#b80f0a"/>`, padL, y-7, barW)
		// % 表示
		fmt.Fprintf(&sb,
			`<text x="%d" y="%d" text-anchor="start" font-size="11" fill="#3a3a3a">%d%%</text>`,
			padL+barW+4, y+4, int(rate*100))
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

// htmlEscape は SVG テキスト用の最小限のエスケープ (<, >, &, ", ')。
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

