package dashboard

// ColorScheme defines the color palette for dashboard charts.
type ColorScheme struct {
	Mode          string   // "cn" | "us"
	ProfitColor   string   // 盈利色
	LossColor     string   // 亏损色
	NeutralColor  string   // 持平色
	GradientStops []string // 5档渐变色阶 (from loss to profit)
}

// CNScheme 中国模式：红涨绿跌
var CNScheme = ColorScheme{
	Mode:          "cn",
	ProfitColor:   "#f04134",
	LossColor:     "#00a854",
	NeutralColor:  "#f5f5f5",
	GradientStops: []string{"#00a854", "#7ec87e", "#f5f5f5", "#e87c7c", "#f04134"},
}

// USScheme 国际模式：绿涨红跌
var USScheme = ColorScheme{
	Mode:          "us",
	ProfitColor:   "#00a854",
	LossColor:     "#f04134",
	NeutralColor:  "#f5f5f5",
	GradientStops: []string{"#f04134", "#e87c7c", "#f5f5f5", "#7ec87e", "#00a854"},
}

// GetColorScheme returns the ColorScheme for the given mode string.
// Defaults to CNScheme if mode is unrecognized.
func GetColorScheme(mode string) ColorScheme {
	switch mode {
	case "us":
		return USScheme
	default:
		return CNScheme
	}
}
