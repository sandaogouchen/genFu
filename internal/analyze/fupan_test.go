package analyze

import (
	"fmt"
	"log"
	"testing"
)

func TestParseFupanBreadth(t *testing.T) {
	html := `<div>个股涨跌图 上涨1304(28%) 停牌12(0%) 下跌3273(71%) 来源：同花顺数据中心</div>`
	up, halt, down, text := parseFupanBreadth(html)
	if up != 1304 || halt != 12 || down != 3273 {
		t.Fatalf("unexpected breadth: up=%d halt=%d down=%d", up, halt, down)
	}
	if text == "" {
		t.Fatalf("expected breadth text")
	}
}

func TestFetchFupanReport(t *testing.T) {
	url := "https://stock.10jqka.com.cn/fupan/"
	ctx := t.Context()
	fupanReport, _ := fetchAndParseFupanURL(ctx, url, "20260213")
	log.Printf("报告Breadth部分：%s", fupanReport.Breadth)
	fmt.Print(fupanReport)
}

func TestParseFupanBreadthFromLegendLabel(t *testing.T) {
	html := `
<div class="legend">
  <span class="legendLabel">上涨 1,304 (28%)</span>
  <span class="legendLabel">平盘 12 (0%)</span>
  <span class="legendLabel">下跌 3,273 (71%)</span>
</div>`
	up, halt, down, text := parseFupanBreadth(html)
	if up != 1304 || halt != 12 || down != 3273 {
		t.Fatalf("unexpected breadth from legendLabel: up=%d halt=%d down=%d", up, halt, down)
	}
	if text == "" {
		t.Fatalf("expected legend breadth text")
	}
}

func TestParseFupanBreadthRejectTinyLegendLabel(t *testing.T) {
	html := `
<div>个股涨跌图</div>
<div class="legend">
  <span class="legendLabel">上涨 66 (83%)</span>
  <span class="legendLabel">下跌 13 (17%)</span>
</div>`
	up, halt, down, text := parseFupanBreadth(html)
	log.Printf("up:%d,halt:%d,down:%d,text:%s", up, halt, down, text)
	if up != 0 || halt != 0 || down != 0 || text != "" {
		t.Fatalf("expected tiny breadth rejected, got up=%d halt=%d down=%d text=%q", up, halt, down, text)
	}
}

func TestParseFupanBreadthFromPieLegendStructure(t *testing.T) {
	html := `
<div class="fp_item_4" id="fp_item_4">
  <div class="fp_item_cnt clearfix">
    <div class="layout_width">
      <div class="flash2 clearfix" id="chart1">
        <div class="pie_legend">
          <table><tbody>
            <tr><td class="legendLabel">上涨<font color="#f83634"><em class="flsh_tip_num">1304</em>(28%)</font></td></tr>
            <tr><td class="legendLabel">停牌<font color="#8a8989"><em class="flsh_tip_num">12</em>(0%)</font></td></tr>
            <tr><td class="legendLabel">下跌<font color="#68b979"><em class="flsh_tip_num">3273</em>(71%)</font></td></tr>
            <tr><td class="legendLabel">来源：同花顺数据中心(不包含科创版)</td></tr>
          </tbody></table>
        </div>
      </div>
    </div>
  </div>
</div>`
	up, halt, down, text := parseFupanBreadth(html)
	if up != 1304 || halt != 12 || down != 3273 {
		t.Fatalf("unexpected pie_legend breadth: up=%d halt=%d down=%d", up, halt, down)
	}
	if text == "" {
		t.Fatalf("expected pie_legend breadth text")
	}
}

func TestParseFupanIndexesFiltersStocks(t *testing.T) {
	html := `<ul class="nav_list">
<li>上证指数 4082.07 -51.95 -1.26%</li>
<li>深证指数 14100.19 -182.81 -1.28%</li>
<li>平安银行 10.91 +0.00 0.00%</li>
<li>恒生指数 26553.53 -479.01 -1.77%</li>
</ul>`
	indexes := parseFupanIndexes(html)
	if len(indexes) != 3 {
		t.Fatalf("expected 3 indexes, got %d", len(indexes))
	}
	if indexes[0].Name != "上证指数" {
		t.Fatalf("unexpected first index name: %s", indexes[0].Name)
	}
	if indexes[1].Name != "深证成指" {
		t.Fatalf("unexpected second index name: %s", indexes[1].Name)
	}
	if indexes[2].Name != "恒生指数" {
		t.Fatalf("unexpected third index name: %s", indexes[2].Name)
	}
}
