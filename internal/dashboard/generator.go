package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"time"
)

// HTMLGenerator generates self-contained HTML dashboard pages.
type HTMLGenerator struct {
	colorScheme ColorScheme
	offline     bool // if true, inline echarts.js (P2)
}

// NewHTMLGenerator creates a new HTMLGenerator with the given color scheme.
func NewHTMLGenerator(scheme ColorScheme) *HTMLGenerator {
	return &HTMLGenerator{colorScheme: scheme}
}

// GenerateHTML writes a full self-contained HTML dashboard to w.
func (g *HTMLGenerator) GenerateHTML(w io.Writer, data DashboardData) error {
	tmpl, err := template.New("dashboard").Funcs(g.templateFuncs()).Parse(dashboardTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	treeMapJSON, _ := json.Marshal(g.buildTreeMapData(data))
	pieJSON, _ := json.Marshal(g.buildPieData(data))
	sunburstJSON, _ := json.Marshal(g.buildSunburstData(data))
	lineJSON, _ := json.Marshal(g.buildLineData(data))
	posTableJSON, _ := json.Marshal(data.Positions)
	gradientJSON, _ := json.Marshal(g.colorScheme.GradientStops)

	ctx := templateContext{
		Title:         "genFu 持仓仪表盘",
		GeneratedAt:   data.GeneratedAt.Format("2006-01-02 15:04"),
		Summary:       data.Summary,
		TreeMapJSON:   template.JS(treeMapJSON),
		PieJSON:       template.JS(pieJSON),
		SunburstJSON:  template.JS(sunburstJSON),
		LineJSON:      template.JS(lineJSON),
		PosTableJSON:  template.JS(posTableJSON),
		GradientJSON:  template.JS(gradientJSON),
		ProfitColor:   g.colorScheme.ProfitColor,
		LossColor:     g.colorScheme.LossColor,
		NeutralColor:  g.colorScheme.NeutralColor,
		ColorMode:     g.colorScheme.Mode,
		EChartsSource: "https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js",
	}

	return tmpl.Execute(w, ctx)
}

// GenerateHTMLString returns the full HTML as a string.
func (g *HTMLGenerator) GenerateHTMLString(data DashboardData) (string, error) {
	var buf bytes.Buffer
	if err := g.GenerateHTML(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type templateContext struct {
	Title         string
	GeneratedAt   string
	Summary       SummaryKPI
	TreeMapJSON   template.JS
	PieJSON       template.JS
	SunburstJSON  template.JS
	LineJSON      template.JS
	PosTableJSON  template.JS
	GradientJSON  template.JS
	ProfitColor   string
	LossColor     string
	NeutralColor  string
	ColorMode     string
	EChartsSource string
}

func (g *HTMLGenerator) templateFuncs() template.FuncMap {
	profitColor := g.colorScheme.ProfitColor
	lossColor := g.colorScheme.LossColor
	return template.FuncMap{
		"formatMoney": func(v float64) string {
			if math.Abs(v) >= 1e8 {
				return fmt.Sprintf("%.2f亿", v/1e8)
			}
			if math.Abs(v) >= 1e4 {
				return fmt.Sprintf("%.2f万", v/1e4)
			}
			return fmt.Sprintf("%.2f", v)
		},
		"formatPnL": func(v float64) string {
			sign := ""
			if v > 0 {
				sign = "+"
			}
			if math.Abs(v) >= 1e8 {
				return fmt.Sprintf("%s%.2f亿", sign, v/1e8)
			}
			if math.Abs(v) >= 1e4 {
				return fmt.Sprintf("%s%.2f万", sign, v/1e4)
			}
			return fmt.Sprintf("%s%.2f", sign, v)
		},
		"formatPct": func(v float64) string {
			sign := ""
			if v > 0 {
				sign = "+"
			}
			return fmt.Sprintf("%s%.2f%%", sign, v*100)
		},
		"pnlClass": func(v float64) string {
			if v > 0 {
				return "profit"
			}
			if v < 0 {
				return "loss"
			}
			return "neutral"
		},
		"pnlColor": func(v float64) string {
			if v > 0 {
				return profitColor
			}
			if v < 0 {
				return lossColor
			}
			return "#888"
		},
	}
}

// --- Chart data builders ---

type treeMapNode struct {
	Name       string         `json:"name"`
	Value      float64        `json:"value,omitempty"`
	ColorValue float64        `json:"colorValue,omitempty"`
	Children   []treeMapNode  `json:"children,omitempty"`
	ItemStyle  *treeMapStyle  `json:"itemStyle,omitempty"`
	Tooltip    *treeMapTooltip `json:"tooltip,omitempty"`
}

type treeMapStyle struct {
	BorderColor string  `json:"borderColor,omitempty"`
	BorderWidth float64 `json:"borderWidth,omitempty"`
}

type treeMapTooltip struct {
	Formatter string `json:"formatter,omitempty"`
}

func (g *HTMLGenerator) buildTreeMapData(data DashboardData) []treeMapNode {
	industryNodes := make(map[string]*treeMapNode)

	for _, pd := range data.Positions {
		industry := pd.Industry
		node, ok := industryNodes[industry]
		if !ok {
			node = &treeMapNode{
				Name: industry,
				ItemStyle: &treeMapStyle{
					BorderColor: "#fff",
					BorderWidth: 2,
				},
			}
			industryNodes[industry] = node
		}

		label := pd.Symbol
		if pd.Name != "" {
			label = pd.Symbol + " " + pd.Name
		}

		child := treeMapNode{
			Name:       label,
			Value:      math.Max(pd.Value, 0.01), // avoid zero area
			ColorValue: pd.PnLPct,
		}
		node.Children = append(node.Children, child)
	}

	result := make([]treeMapNode, 0, len(industryNodes))
	for _, n := range industryNodes {
		result = append(result, *n)
	}
	return result
}

type pieItem struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

func (g *HTMLGenerator) buildPieData(data DashboardData) []pieItem {
	items := make([]pieItem, 0, len(data.IndustryBreak))
	otherValue := 0.0

	for _, ig := range data.IndustryBreak {
		if ig.Weight < 0.03 {
			otherValue += ig.TotalValue
			continue
		}
		items = append(items, pieItem{Name: ig.Industry, Value: math.Round(ig.TotalValue*100) / 100})
	}
	if otherValue > 0 {
		items = append(items, pieItem{Name: "其他", Value: math.Round(otherValue*100) / 100})
	}
	return items
}

type sunburstNode struct {
	Name     string         `json:"name"`
	Value    float64        `json:"value,omitempty"`
	Children []sunburstNode `json:"children,omitempty"`
}

func (g *HTMLGenerator) buildSunburstData(data DashboardData) []sunburstNode {
	result := make([]sunburstNode, 0, len(data.AssetBreak))
	for _, ag := range data.AssetBreak {
		assetNode := sunburstNode{Name: g.assetTypeName(ag.AssetType)}

		for _, ig := range ag.Industries {
			indNode := sunburstNode{Name: ig.Industry}
			otherValue := 0.0

			for _, pd := range ig.Positions {
				if pd.Weight < 0.02 {
					otherValue += pd.Value
					continue
				}
				indNode.Children = append(indNode.Children, sunburstNode{
					Name:  pd.Symbol,
					Value: math.Round(pd.Value*100) / 100,
				})
			}
			if otherValue > 0 {
				indNode.Children = append(indNode.Children, sunburstNode{
					Name:  "其他",
					Value: math.Round(otherValue*100) / 100,
				})
			}
			if len(indNode.Children) == 0 {
				indNode.Value = math.Round(ig.TotalValue*100) / 100
			}
			assetNode.Children = append(assetNode.Children, indNode)
		}
		result = append(result, assetNode)
	}
	return result
}

type lineData struct {
	Dates      []string  `json:"dates"`
	Values     []float64 `json:"values"`
	Costs      []float64 `json:"costs"`
}

func (g *HTMLGenerator) buildLineData(data DashboardData) lineData {
	ld := lineData{}
	for _, vp := range data.Valuations {
		ld.Dates = append(ld.Dates, vp.Date)
		ld.Values = append(ld.Values, math.Round(vp.TotalValue*100)/100)
		ld.Costs = append(ld.Costs, math.Round(vp.TotalCost*100)/100)
	}
	return ld
}

func (g *HTMLGenerator) assetTypeName(at string) string {
	switch at {
	case "stock":
		return "股票"
	case "fund":
		return "基金"
	case "bond":
		return "债券"
	case "cash":
		return "现金"
	case "etf":
		return "ETF"
	default:
		return at
	}
}

// GenerateToFile writes the HTML to the given path.
func (g *HTMLGenerator) GenerateToFile(data DashboardData, path string) error {
	_ = path
	_ = time.Now() // placeholder for future file write
	return nil      // will be implemented in handler
}
