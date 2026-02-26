package tool

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeSecID(t *testing.T) {
	cases := map[string]string{
		"600000":   "1.600000",
		"000001":   "0.000001",
		"300001":   "0.300001",
		"SH600000": "1.600000",
		"SZ000001": "0.000001",
		"1.600000": "1.600000",
	}
	for input, expected := range cases {
		if got := normalizeSecID(input); got != expected {
			t.Fatalf("normalizeSecID(%s)=%s", input, got)
		}
	}
}

func TestScaleHelpers(t *testing.T) {
	// 东方财富API返回的价格已经是元，不需要转换
	if scalePrice(9.89) != 9.89 {
		t.Fatalf("scalePrice")
	}
	// 涨跌幅需要除以100
	if scalePercent(123) != 1.23 {
		t.Fatalf("scalePercent")
	}
	// 东方财富API返回的金额已经是元，不需要转换
	if scaleAmount(100) != 100 {
		t.Fatalf("scaleAmount")
	}
}

func TestFallbackHostsForEndpoint(t *testing.T) {
	realtime := fallbackHostsForEndpoint("https://push2.eastmoney.com/api/qt/clist/get")
	if len(realtime) == 0 {
		t.Fatalf("expected realtime hosts, got empty")
	}
	for _, host := range realtime {
		if strings.Contains(host, "push2his.eastmoney.com") {
			t.Fatalf("realtime endpoint should not include push2his host: %v", realtime)
		}
	}

	history := fallbackHostsForEndpoint("https://push2his.eastmoney.com/api/qt/stock/kline/get")
	if len(history) == 0 {
		t.Fatalf("expected history hosts, got empty")
	}
	foundHistoryHost := false
	for _, host := range history {
		if strings.Contains(host, "push2his.eastmoney.com") {
			foundHistoryHost = true
			break
		}
	}
	if !foundHistoryHost {
		t.Fatalf("history endpoint should include push2his host: %v", history)
	}
}

func TestCandidateHostsForEndpoint(t *testing.T) {
	endpoint := "https://82.push2.eastmoney.com/api/qt/clist/get"
	hosts := candidateHostsForEndpoint(endpoint)
	if len(hosts) == 0 {
		t.Fatalf("expected non-empty candidate hosts")
	}
	if hosts[0] != "https://82.push2.eastmoney.com" {
		t.Fatalf("expected original host first, got=%v", hosts)
	}
	seen := map[string]struct{}{}
	for _, host := range hosts {
		if _, ok := seen[host]; ok {
			t.Fatalf("unexpected duplicate host list: %v", hosts)
		}
		seen[host] = struct{}{}
	}
}

func TestShouldUseUTLSBackend(t *testing.T) {
	cases := []struct {
		endpoint string
		want     bool
	}{
		{endpoint: "https://push2.eastmoney.com/api/qt/clist/get", want: true},
		{endpoint: "https://push2.eastmoney.com/api/qt/stock/get", want: true},
		{endpoint: "https://push2his.eastmoney.com/api/qt/stock/kline/get", want: true},
		{endpoint: "https://push2.eastmoney.com/api/qt/stock/trends2/get", want: false},
		{endpoint: "https://fundf10.eastmoney.com/F10DataApi.aspx", want: false},
	}
	for _, tc := range cases {
		if got := shouldUseUTLSBackend(tc.endpoint); got != tc.want {
			t.Fatalf("shouldUseUTLSBackend(%s)=%v want=%v", tc.endpoint, got, tc.want)
		}
	}
}

func TestEastmoneyEndpointLabel(t *testing.T) {
	got := eastmoneyEndpointLabel("https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=200")
	want := "https://push2.eastmoney.com/api/qt/clist/get"
	if got != want {
		t.Fatalf("unexpected endpoint label: got=%s want=%s", got, want)
	}
}

func TestNormalizeEastMoneyOptions(t *testing.T) {
	opts := normalizeEastMoneyOptions(EastMoneyOptions{})
	if opts.Timeout <= 0 {
		t.Fatalf("expected positive timeout")
	}
	if opts.MaxRetries <= 0 {
		t.Fatalf("expected positive max retries")
	}
	if opts.MinInterval <= 0 {
		t.Fatalf("expected positive min interval")
	}
	if strings.TrimSpace(opts.Referer) == "" {
		t.Fatalf("expected non-empty referer")
	}
	if strings.TrimSpace(opts.UserAgent) == "" {
		t.Fatalf("expected non-empty user agent")
	}
	custom := normalizeEastMoneyOptions(EastMoneyOptions{
		Timeout:     9 * time.Second,
		MaxRetries:  2,
		MinInterval: 50 * time.Millisecond,
		Referer:     "https://quote.eastmoney.com/",
		UserAgent:   "test-agent",
	})
	if custom.Timeout != 9*time.Second || custom.MaxRetries != 2 || custom.MinInterval != 50*time.Millisecond {
		t.Fatalf("expected custom duration/retry settings to be kept")
	}
	if custom.Referer != "https://quote.eastmoney.com/" {
		t.Fatalf("unexpected referer: %s", custom.Referer)
	}
	if custom.UserAgent != "test-agent" {
		t.Fatalf("unexpected user agent: %s", custom.UserAgent)
	}
}

func TestParseFundSearchResponseFromDatas(t *testing.T) {
	body := []byte(`{"Datas":["000001,华夏成长混合,extra","000002,中欧医疗健康混合A","bad-row"]}`)
	items, err := parseFundSearchResponse(body, 10)
	if err != nil {
		t.Fatalf("parseFundSearchResponse err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Code != "000001" || items[0].Name != "华夏成长混合" || items[0].Type != "fund" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].Code != "000002" || items[1].Name != "中欧医疗健康混合A" || items[1].Type != "fund" {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
}

func TestParseFundSearchResponseJSONPAndLimit(t *testing.T) {
	body := []byte(`jQuery123({"Datas":["000001,华夏成长混合","000002,中欧医疗健康混合A","000003,易方达蓝筹精选"]})`)
	items, err := parseFundSearchResponse(body, 2)
	if err != nil {
		t.Fatalf("parseFundSearchResponse err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Code != "000001" || items[1].Code != "000002" {
		t.Fatalf("unexpected item order: %+v", items)
	}
}

func TestParseFundSearchResponseFallbackDataObject(t *testing.T) {
	body := []byte(`{"Data":[{"Code":"161725","Name":"招商中证白酒指数"},{"FCode":"110011","ShortName":"易方达中小盘"}]}`)
	items, err := parseFundSearchResponse(body, 10)
	if err != nil {
		t.Fatalf("parseFundSearchResponse err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Code != "161725" || items[0].Name != "招商中证白酒指数" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].Code != "110011" || items[1].Name != "易方达中小盘" {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
}

func TestParseFundSearchResponseSkipsBadRows(t *testing.T) {
	body := []byte(`{"Datas":["bad-row","000001,华夏成长混合",""],"Data":[{"Code":"","Name":"skip"},{"Code":"110022","Name":"易方达消费行业"}]}`)
	items, err := parseFundSearchResponse(body, 10)
	if err != nil {
		t.Fatalf("parseFundSearchResponse err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Code != "000001" || items[1].Code != "110022" {
		t.Fatalf("unexpected parsed codes: %+v", items)
	}
}

func TestParseFundSearchResponseDatasObjectRows(t *testing.T) {
	body := []byte(`{"Datas":[{"Code":"519674","Name":"银河创新成长混合A"},{"Code":"161725","Name":"招商中证白酒指数"}]}`)
	items, err := parseFundSearchResponse(body, 10)
	if err != nil {
		t.Fatalf("parseFundSearchResponse err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Code != "519674" || items[0].Name != "银河创新成长混合A" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].Code != "161725" || items[1].Name != "招商中证白酒指数" {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
}
