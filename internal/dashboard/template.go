package dashboard

// dashboardTemplate is the self-contained HTML template for the dashboard page.
// It uses ECharts from CDN and receives chart data as JSON injected by Go.
const dashboardTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<script src="{{.EChartsSource}}"></script>
<style>
  :root {
    --profit-color: {{.ProfitColor}};
    --loss-color: {{.LossColor}};
    --neutral-color: {{.NeutralColor}};
    --bg: #0d1117; --card-bg: #161b22; --text: #c9d1d9; --text-muted: #8b949e;
    --border: #30363d;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
         background: var(--bg); color: var(--text); padding: 16px; }
  .header { text-align: center; padding: 12px 0 20px; }
  .header h1 { font-size: 1.5rem; font-weight: 600; }
  .header .subtitle { color: var(--text-muted); font-size: 0.85rem; margin-top: 4px; }
  .disclaimer { text-align: center; color: var(--text-muted); font-size: 0.75rem;
                 padding: 8px; border-top: 1px solid var(--border); margin-top: 16px; }

  /* KPI Panel */
  .kpi-panel { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
               gap: 12px; margin-bottom: 20px; }
  .kpi-card { background: var(--card-bg); border: 1px solid var(--border);
              border-radius: 8px; padding: 16px; text-align: center; }
  .kpi-label { font-size: 0.8rem; color: var(--text-muted); margin-bottom: 6px; }
  .kpi-value { font-size: 1.4rem; font-weight: 700; }
  .kpi-sub { font-size: 0.85rem; margin-top: 2px; }
  .profit { color: var(--profit-color); }
  .loss { color: var(--loss-color); }
  .neutral { color: var(--text-muted); }

  /* Chart containers */
  .chart-section { background: var(--card-bg); border: 1px solid var(--border);
                   border-radius: 8px; padding: 16px; margin-bottom: 16px; }
  .chart-section h2 { font-size: 1rem; font-weight: 600; margin-bottom: 12px;
                       padding-bottom: 8px; border-bottom: 1px solid var(--border); }
  .chart-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
  .chart-box { min-height: 400px; }
  .chart-box-half { min-height: 360px; }

  /* Position table */
  .pos-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  .pos-table th { text-align: left; padding: 8px 10px; border-bottom: 2px solid var(--border);
                   color: var(--text-muted); font-weight: 600; font-size: 0.8rem; }
  .pos-table td { padding: 8px 10px; border-bottom: 1px solid var(--border); }
  .pos-table tr:hover td { background: rgba(255,255,255,0.03); }
  .pos-table .num { text-align: right; font-variant-numeric: tabular-nums; }

  /* Responsive */
  @media (max-width: 768px) {
    .chart-row { grid-template-columns: 1fr; }
    .kpi-panel { grid-template-columns: repeat(2, 1fr); }
    .kpi-value { font-size: 1.1rem; }
    body { padding: 8px; }
  }
</style>
</head>
<body>

<div class="header">
  <h1>{{.Title}}</h1>
  <div class="subtitle">生成时间: {{.GeneratedAt}} | 配色: {{if eq .ColorMode "cn"}}A股模式（红涨绿跌）{{else}}国际模式（绿涨红跌）{{end}}</div>
</div>

<!-- KPI Panel -->
<div class="kpi-panel">
  <div class="kpi-card">
    <div class="kpi-label">总资产</div>
    <div class="kpi-value">¥{{formatMoney .Summary.TotalValue}}</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-label">总盈亏</div>
    <div class="kpi-value {{pnlClass .Summary.TotalPnL}}">
      {{formatPnL .Summary.TotalPnL}}
    </div>
    <div class="kpi-sub {{pnlClass .Summary.TotalPnL}}">{{formatPct .Summary.TotalPnLPct}}</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-label">今日盈亏</div>
    <div class="kpi-value {{pnlClass .Summary.DailyPnL}}">
      {{formatPnL .Summary.DailyPnL}}
    </div>
    <div class="kpi-sub {{pnlClass .Summary.DailyPnL}}">{{formatPct .Summary.DailyPnLPct}}</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-label">持仓数</div>
    <div class="kpi-value">{{.Summary.PositionCount}}</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-label">现金余额</div>
    <div class="kpi-value">¥{{formatMoney .Summary.CashBalance}}</div>
  </div>
</div>

<!-- TreeMap: PnL Heatmap -->
<div class="chart-section">
  <h2>盈亏热力图</h2>
  <div id="treemap" class="chart-box"></div>
</div>

<!-- Pie + Sunburst row -->
<div class="chart-row">
  <div class="chart-section">
    <h2>行业分布</h2>
    <div id="pie" class="chart-box-half"></div>
  </div>
  <div class="chart-section">
    <h2>资产配置</h2>
    <div id="sunburst" class="chart-box-half"></div>
  </div>
</div>

<!-- Valuation Line -->
<div class="chart-section" id="valuation-section" style="display:none">
  <h2>估值趋势</h2>
  <div id="valuation" class="chart-box-half"></div>
</div>

<!-- Position Detail Table -->
<div class="chart-section">
  <h2>持仓明细</h2>
  <div style="overflow-x:auto">
    <table class="pos-table" id="pos-table">
      <thead>
        <tr>
          <th>标的</th><th>名称</th><th>行业</th><th class="num">数量</th>
          <th class="num">成本价</th><th class="num">市价</th>
          <th class="num">盈亏</th><th class="num">收益率</th><th class="num">占比</th>
        </tr>
      </thead>
      <tbody></tbody>
    </table>
  </div>
</div>

<div class="disclaimer">
  ⚠️ 此文件包含个人投资数据，请勿分享到公开网络。由 genFu 自动生成。
</div>

<script>
(function() {
  var profitColor = '{{.ProfitColor}}';
  var lossColor   = '{{.LossColor}}';
  var neutralColor = '{{.NeutralColor}}';
  var gradient = {{.GradientJSON}};

  var treeMapData = {{.TreeMapJSON}};
  var pieData     = {{.PieJSON}};
  var sunburstData = {{.SunburstJSON}};
  var lineData    = {{.LineJSON}};
  var posData     = {{.PosTableJSON}};

  // --- Helper ---
  function fmtMoney(v) {
    if (Math.abs(v) >= 1e8) return (v/1e8).toFixed(2) + '亿';
    if (Math.abs(v) >= 1e4) return (v/1e4).toFixed(2) + '万';
    return v.toFixed(2);
  }
  function fmtPct(v) {
    var s = v > 0 ? '+' : '';
    return s + (v * 100).toFixed(2) + '%';
  }
  function pnlColor(v) { return v > 0 ? profitColor : (v < 0 ? lossColor : '#888'); }

  // --- TreeMap ---
  var treemapChart = echarts.init(document.getElementById('treemap'), 'dark');
  treemapChart.setOption({
    tooltip: {
      formatter: function(info) {
        if (!info.data || info.data.children) {
          return info.name + '<br/>市值: ¥' + fmtMoney(info.value);
        }
        var d = info.data;
        return '<b>' + d.name + '</b><br/>' +
          '市值: ¥' + fmtMoney(d.value) + '<br/>' +
          '收益率: ' + fmtPct(d.colorValue || 0);
      }
    },
    visualMap: {
      type: 'continuous', min: -0.30, max: 0.30, calculable: true, orient: 'horizontal',
      left: 'center', bottom: 10,
      inRange: { color: gradient },
      text: ['盈利', '亏损'],
      textStyle: { color: '#c9d1d9' }
    },
    series: [{
      type: 'treemap',
      data: treeMapData,
      width: '100%', height: '85%',
      roam: false,
      nodeClick: false,
      breadcrumb: { show: true, emptyItemWidth: 25 },
      label: {
        show: true,
        formatter: function(p) {
          if (p.data && p.data.colorValue !== undefined) {
            return p.data.name + '\n' + fmtPct(p.data.colorValue);
          }
          return p.name;
        },
        fontSize: 12, color: '#fff'
      },
      upperLabel: { show: true, height: 24, color: '#fff', fontSize: 12 },
      levels: [
        { itemStyle: { borderColor: '#555', borderWidth: 3, gapWidth: 3 },
          upperLabel: { show: true } },
        { itemStyle: { borderColor: '#777', borderWidth: 1, gapWidth: 1 },
          colorSaturation: [0.3, 0.8] }
      ]
    }]
  });

  // --- Pie ---
  var pieChart = echarts.init(document.getElementById('pie'), 'dark');
  pieChart.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: ¥{c} ({d}%)' },
    legend: { type: 'scroll', orient: 'vertical', right: 10, top: 20, bottom: 20, textStyle: { color: '#c9d1d9' } },
    series: [{
      type: 'pie', radius: ['35%', '65%'], center: ['40%', '50%'],
      data: pieData,
      emphasis: { itemStyle: { shadowBlur: 10, shadowOffsetX: 0, shadowColor: 'rgba(0,0,0,0.5)' } },
      label: { formatter: '{b}\n{d}%', fontSize: 11, color: '#c9d1d9' },
      itemStyle: { borderRadius: 6, borderColor: '#161b22', borderWidth: 2 }
    }]
  });

  // --- Sunburst ---
  var sunburstChart = echarts.init(document.getElementById('sunburst'), 'dark');
  sunburstChart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: function(p) {
        return p.name + (p.value ? '<br/>¥' + fmtMoney(p.value) : '');
      }
    },
    series: [{
      type: 'sunburst',
      data: sunburstData,
      radius: ['10%', '90%'],
      sort: 'desc',
      emphasis: { focus: 'ancestor' },
      levels: [
        {},
        { r0: '10%', r: '35%',
          itemStyle: { borderWidth: 2 },
          label: { rotate: 'tangential', fontSize: 12 } },
        { r0: '35%', r: '60%',
          label: { align: 'right', fontSize: 10 } },
        { r0: '60%', r: '90%',
          label: { position: 'outside', padding: 3, silent: false, fontSize: 9 },
          itemStyle: { borderWidth: 1 } }
      ]
    }]
  });

  // --- Valuation Line (only if data exists) ---
  if (lineData && lineData.dates && lineData.dates.length > 1) {
    document.getElementById('valuation-section').style.display = 'block';
    var lineChart = echarts.init(document.getElementById('valuation'), 'dark');
    lineChart.setOption({
      tooltip: { trigger: 'axis',
        formatter: function(ps) {
          var s = ps[0].axisValueLabel + '<br/>';
          ps.forEach(function(p) { s += p.marker + p.seriesName + ': ¥' + fmtMoney(p.value) + '<br/>'; });
          return s;
        }
      },
      legend: { data: ['总市值', '总成本'], textStyle: { color: '#c9d1d9' } },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: { type: 'category', data: lineData.dates, boundaryGap: false },
      yAxis: { type: 'value',
        axisLabel: { formatter: function(v) { return '¥' + fmtMoney(v); } }
      },
      series: [
        { name: '总市值', type: 'line', data: lineData.values, smooth: true,
          areaStyle: { opacity: 0.15 }, lineStyle: { width: 2 } },
        { name: '总成本', type: 'line', data: lineData.costs, smooth: true,
          lineStyle: { type: 'dashed', width: 1.5 } }
      ]
    });
    window.addEventListener('resize', function() { lineChart.resize(); });
  }

  // --- Position Table ---
  var tbody = document.querySelector('#pos-table tbody');
  posData.forEach(function(p) {
    var tr = document.createElement('tr');
    var pcolor = pnlColor(p.pnl);
    tr.innerHTML =
      '<td>' + p.symbol + '</td>' +
      '<td>' + (p.name || '-') + '</td>' +
      '<td>' + (p.industry || '-') + '</td>' +
      '<td class="num">' + p.quantity.toFixed(2) + '</td>' +
      '<td class="num">' + p.avg_cost.toFixed(2) + '</td>' +
      '<td class="num">' + p.market_price.toFixed(2) + '</td>' +
      '<td class="num" style="color:' + pcolor + '">' + (p.pnl>0?'+':'') + fmtMoney(p.pnl) + '</td>' +
      '<td class="num" style="color:' + pcolor + '">' + fmtPct(p.pnl_pct) + '</td>' +
      '<td class="num">' + (p.weight * 100).toFixed(1) + '%</td>';
    tbody.appendChild(tr);
  });

  // --- Responsive resize ---
  window.addEventListener('resize', function() {
    treemapChart.resize();
    pieChart.resize();
    sunburstChart.resize();
  });
})();
</script>
</body>
</html>` + "\n"