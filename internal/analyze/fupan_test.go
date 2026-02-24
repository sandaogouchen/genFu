package analyze

import "testing"

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
