package financial

import "strings"

// CategoryNames 公告分类码 → 中文名称映射
var CategoryNames = map[string]string{
	"category_ndbg_szsh":  "年度报告",
	"category_bndbg_szsh": "半年度报告",
	"category_yjdbg_szsh": "一季度报告",
	"category_sjdbg_szsh": "三季度报告",
	"category_yjyg_szsh":  "业绩预告",
	"category_yjkb_szsh":  "业绩快报",
	"category_cxsx_szsh":  "持续经营",
	"category_gszl_szsh":  "公司治理",
	"category_gddh_szsh":  "股东大会",
	"category_dshgg_szsh": "董事会公告",
	"category_jshgg_szsh": "监事会公告",
	"category_rcjy_szsh":  "日常经营",
	"category_zcdx_szsh":  "重大事项",
	"category_bgcz_szsh":  "并购重组",
	"category_zpfx_szsh":  "增发配股",
	"category_gqbd_szsh":  "股权变动",
	"category_fhps_szsh":  "分红派送",
	"category_qtgg_szsh":  "其他公告",
}

// categoryAliases 自然语言别名 → 分类码映射
// 当 Agent 传入中文名称时，自动解析为 CNInfo 分类码
var categoryAliases = map[string]string{
	"年报":     "category_ndbg_szsh",
	"年度报告": "category_ndbg_szsh",
	"半年报":   "category_bndbg_szsh",
	"半年度报告": "category_bndbg_szsh",
	"中报":     "category_bndbg_szsh",
	"一季报":   "category_yjdbg_szsh",
	"一季度报告": "category_yjdbg_szsh",
	"三季报":   "category_sjdbg_szsh",
	"三季度报告": "category_sjdbg_szsh",
	"季报":     "category_yjdbg_szsh;category_sjdbg_szsh",
	"业绩预告": "category_yjyg_szsh",
	"业绩快报": "category_yjkb_szsh",
	"持续经营": "category_cxsx_szsh",
	"退市":     "category_cxsx_szsh",
	"ST":       "category_cxsx_szsh",
	"公司治理": "category_gszl_szsh",
	"股东大会": "category_gddh_szsh",
	"董事会":   "category_dshgg_szsh",
	"董事会公告": "category_dshgg_szsh",
	"监事会":   "category_jshgg_szsh",
	"监事会公告": "category_jshgg_szsh",
	"日常经营": "category_rcjy_szsh",
	"关联交易": "category_rcjy_szsh",
	"担保":     "category_rcjy_szsh",
	"重大事项": "category_zcdx_szsh",
	"重大合同": "category_zcdx_szsh",
	"诉讼":     "category_zcdx_szsh",
	"并购":     "category_bgcz_szsh",
	"重组":     "category_bgcz_szsh",
	"并购重组": "category_bgcz_szsh",
	"资产收购": "category_bgcz_szsh",
	"定增":     "category_zpfx_szsh",
	"增发":     "category_zpfx_szsh",
	"配股":     "category_zpfx_szsh",
	"增发配股": "category_zpfx_szsh",
	"定向增发": "category_zpfx_szsh",
	"可转债":   "category_zpfx_szsh",
	"股权变动": "category_gqbd_szsh",
	"股权激励": "category_gqbd_szsh",
	"增减持":   "category_gqbd_szsh",
	"分红":     "category_fhps_szsh",
	"分红派送": "category_fhps_szsh",
	"送转":     "category_fhps_szsh",
	"其他":     "category_qtgg_szsh",
	"其他公告": "category_qtgg_szsh",
}

// ResolveCategoryCode 将用户输入（分类码或中文别名）解析为有效的 CNInfo 分类码
// 如果输入已经是标准分类码（以 "category_" 开头），直接返回
// 否则尝试从别名表中匹配
// 返回空字符串表示不限分类
func ResolveCategoryCode(input string) string {
	if input == "" {
		return ""
	}

	// 已经是标准分类码
	if strings.HasPrefix(input, "category_") {
		if _, ok := CategoryNames[input]; ok {
			return input
		}
		// 可能是分号分隔的多分类
		return input
	}

	// 精确匹配别名
	if code, ok := categoryAliases[input]; ok {
		return code
	}

	// 尝试去除空格后匹配
	trimmed := strings.TrimSpace(input)
	if code, ok := categoryAliases[trimmed]; ok {
		return code
	}

	// 无法匹配，返回原始输入让 API 自行处理
	return input
}

// GetAllCategoryTypes 返回全部分类类型列表（供 get_announcement_types Action 使用）
func GetAllCategoryTypes() []map[string]string {
	result := make([]map[string]string, 0, len(CategoryNames))
	// 按固定顺序返回
	orderedCodes := []string{
		"category_ndbg_szsh",
		"category_bndbg_szsh",
		"category_yjdbg_szsh",
		"category_sjdbg_szsh",
		"category_yjyg_szsh",
		"category_yjkb_szsh",
		"category_cxsx_szsh",
		"category_gszl_szsh",
		"category_gddh_szsh",
		"category_dshgg_szsh",
		"category_jshgg_szsh",
		"category_rcjy_szsh",
		"category_zcdx_szsh",
		"category_bgcz_szsh",
		"category_zpfx_szsh",
		"category_gqbd_szsh",
		"category_fhps_szsh",
		"category_qtgg_szsh",
	}
	for _, code := range orderedCodes {
		name := CategoryNames[code]
		aliases := findAliases(code)
		result = append(result, map[string]string{
			"code":    code,
			"name":    name,
			"aliases": strings.Join(aliases, ", "),
		})
	}
	return result
}

// findAliases 查找某个分类码的所有别名
func findAliases(code string) []string {
	var aliases []string
	seen := make(map[string]bool)
	for alias, c := range categoryAliases {
		if c == code && !seen[alias] {
			aliases = append(aliases, alias)
			seen[alias] = true
		}
	}
	return aliases
}