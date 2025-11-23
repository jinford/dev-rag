package quality

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
)

// ReportGenerator はHTMLレポートを生成するサービスです
type ReportGenerator struct {
	metricsCalc   MetricsCalculatorInterface
	freshnessCalc FreshnessCalculatorInterface
}

// NewReportGenerator は新しいReportGeneratorを作成します
func NewReportGenerator(
	metricsCalc MetricsCalculatorInterface,
	freshnessCalc FreshnessCalculatorInterface,
) *ReportGenerator {
	return &ReportGenerator{
		metricsCalc:   metricsCalc,
		freshnessCalc: freshnessCalc,
	}
}

// ReportData はHTMLレポートに表示するデータを表します
type ReportData struct {
	GeneratedAt      string
	QualityMetrics   *models.QualityMetrics
	FreshnessReport  *models.FreshnessReport
	SeverityData     string // JSON string for Chart.js
	TrendData        string // JSON string for Chart.js
	FreshnessDistData string // JSON string for Chart.js
}

// GenerateHTMLReport はHTMLレポートを生成します
func (rg *ReportGenerator) GenerateHTMLReport(outputPath string) error {
	// メトリクスの取得
	metrics, err := rg.metricsCalc.CalculateMetrics()
	if err != nil {
		return fmt.Errorf("failed to calculate metrics: %w", err)
	}

	// 鮮度レポートの取得
	freshnessReport, err := rg.freshnessCalc.CalculateFreshness(30) // デフォルト30日閾値
	if err != nil {
		return fmt.Errorf("failed to calculate freshness: %w", err)
	}

	// レポートデータの準備
	reportData, err := rg.prepareReportData(metrics, freshnessReport)
	if err != nil {
		return fmt.Errorf("failed to prepare report data: %w", err)
	}

	// HTMLの生成
	html, err := rg.renderHTML(reportData)
	if err != nil {
		return fmt.Errorf("failed to render HTML: %w", err)
	}

	// ファイルに書き込み
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	return nil
}

// prepareReportData はレポートデータを準備します
func (rg *ReportGenerator) prepareReportData(
	metrics *models.QualityMetrics,
	freshnessReport *models.FreshnessReport,
) (*ReportData, error) {
	// Severity別データをJSONに変換
	severityData := map[string]int{
		"critical": metrics.BySeverity[models.QualitySeverityCritical],
		"high":     metrics.BySeverity[models.QualitySeverityHigh],
		"medium":   metrics.BySeverity[models.QualitySeverityMedium],
		"low":      metrics.BySeverity[models.QualitySeverityLow],
	}
	severityJSON, err := json.Marshal(severityData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal severity data: %w", err)
	}

	// トレンドデータをJSONに変換
	trendJSON, err := json.Marshal(metrics.RecentTrend)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trend data: %w", err)
	}

	// 鮮度分布データをJSONに変換（ヒストグラム用）
	freshnessDistribution := rg.calculateFreshnessDistribution(freshnessReport)
	freshnessDistJSON, err := json.Marshal(freshnessDistribution)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal freshness distribution: %w", err)
	}

	return &ReportData{
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
		QualityMetrics:    metrics,
		FreshnessReport:   freshnessReport,
		SeverityData:      string(severityJSON),
		TrendData:         string(trendJSON),
		FreshnessDistData: string(freshnessDistJSON),
	}, nil
}

// calculateFreshnessDistribution は鮮度の分布を計算します（ヒストグラム用）
func (rg *ReportGenerator) calculateFreshnessDistribution(report *models.FreshnessReport) map[string]int {
	distribution := map[string]int{
		"0-7 days":   0,
		"8-14 days":  0,
		"15-30 days": 0,
		"30+ days":   0,
	}

	for _, chunk := range report.StaleChunkDetails {
		switch {
		case chunk.FreshnessDays <= 7:
			distribution["0-7 days"]++
		case chunk.FreshnessDays <= 14:
			distribution["8-14 days"]++
		case chunk.FreshnessDays <= 30:
			distribution["15-30 days"]++
		default:
			distribution["30+ days"]++
		}
	}

	return distribution
}

// renderHTML はHTMLテンプレートをレンダリングします
func (rg *ReportGenerator) renderHTML(data *ReportData) (string, error) {
	tmpl := `<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>品質ダッシュボード - dev-rag</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        body { padding: 20px; background-color: #f8f9fa; }
        .card { margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-card { text-align: center; padding: 20px; }
        .metric-value { font-size: 2.5rem; font-weight: bold; color: #0d6efd; }
        .metric-label { font-size: 0.9rem; color: #6c757d; text-transform: uppercase; }
        .chart-container { position: relative; height: 300px; }
        h1 { color: #212529; margin-bottom: 30px; }
        .generated-at { color: #6c757d; font-size: 0.9rem; margin-bottom: 30px; }
    </style>
</head>
<body>
    <div class="container-fluid">
        <h1>品質ダッシュボード</h1>
        <p class="generated-at">生成日時: {{.GeneratedAt}}</p>

        <!-- サマリーメトリクス -->
        <div class="row">
            <div class="col-md-3">
                <div class="card metric-card">
                    <div class="metric-value">{{.QualityMetrics.TotalNotes}}</div>
                    <div class="metric-label">総品質ノート数</div>
                </div>
            </div>
            <div class="col-md-3">
                <div class="card metric-card">
                    <div class="metric-value">{{.QualityMetrics.OpenNotes}}</div>
                    <div class="metric-label">未解決ノート数</div>
                </div>
            </div>
            <div class="col-md-3">
                <div class="card metric-card">
                    <div class="metric-value">{{.QualityMetrics.ResolvedNotes}}</div>
                    <div class="metric-label">解決済みノート数</div>
                </div>
            </div>
            <div class="col-md-3">
                <div class="card metric-card">
                    <div class="metric-value">{{printf "%.1f" .QualityMetrics.AverageFreshnessDays}}</div>
                    <div class="metric-label">平均鮮度（日数）</div>
                </div>
            </div>
        </div>

        <!-- グラフエリア -->
        <div class="row">
            <!-- Severity別内訳（円グラフ） -->
            <div class="col-md-6">
                <div class="card">
                    <div class="card-body">
                        <h5 class="card-title">深刻度別の内訳</h5>
                        <div class="chart-container">
                            <canvas id="severityChart"></canvas>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 鮮度分布（ヒストグラム） -->
            <div class="col-md-6">
                <div class="card">
                    <div class="card-body">
                        <h5 class="card-title">インデックス鮮度の分布</h5>
                        <div class="chart-container">
                            <canvas id="freshnessChart"></canvas>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <div class="row">
            <!-- 品質メトリクスの時系列グラフ -->
            <div class="col-md-12">
                <div class="card">
                    <div class="card-body">
                        <h5 class="card-title">品質メトリクスの推移</h5>
                        <div class="chart-container">
                            <canvas id="trendChart"></canvas>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- 鮮度詳細テーブル -->
        <div class="row">
            <div class="col-md-12">
                <div class="card">
                    <div class="card-body">
                        <h5 class="card-title">古いチャンクの詳細 (鮮度閾値: {{.FreshnessReport.FreshnessThreshold}}日)</h5>
                        <p class="text-muted">総チャンク数: {{.FreshnessReport.TotalChunks}}, 古いチャンク数: {{.FreshnessReport.StaleChunks}}</p>
                        {{if gt .FreshnessReport.StaleChunks 0}}
                        <div class="table-responsive">
                            <table class="table table-striped table-sm">
                                <thead>
                                    <tr>
                                        <th>ファイルパス</th>
                                        <th>チャンクKey</th>
                                        <th>鮮度（日数）</th>
                                        <th>最終更新</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {{range .FreshnessReport.StaleChunkDetails}}
                                    <tr>
                                        <td>{{.FilePath}}</td>
                                        <td>{{.ChunkKey}}</td>
                                        <td>{{.FreshnessDays}}</td>
                                        <td>{{.LastUpdated.Format "2006-01-02"}}</td>
                                    </tr>
                                    {{end}}
                                </tbody>
                            </table>
                        </div>
                        {{else}}
                        <p class="text-success">すべてのチャンクが最新です！</p>
                        {{end}}
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        // Severity別円グラフ
        const severityData = {{.SeverityData}};
        new Chart(document.getElementById('severityChart'), {
            type: 'pie',
            data: {
                labels: ['Critical', 'High', 'Medium', 'Low'],
                datasets: [{
                    data: [
                        severityData.critical,
                        severityData.high,
                        severityData.medium,
                        severityData.low
                    ],
                    backgroundColor: [
                        'rgba(220, 53, 69, 0.8)',
                        'rgba(255, 193, 7, 0.8)',
                        'rgba(0, 123, 255, 0.8)',
                        'rgba(40, 167, 69, 0.8)'
                    ]
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'bottom' }
                }
            }
        });

        // 鮮度分布ヒストグラム
        const freshnessDistData = {{.FreshnessDistData}};
        new Chart(document.getElementById('freshnessChart'), {
            type: 'bar',
            data: {
                labels: ['0-7 days', '8-14 days', '15-30 days', '30+ days'],
                datasets: [{
                    label: 'チャンク数',
                    data: [
                        freshnessDistData['0-7 days'],
                        freshnessDistData['8-14 days'],
                        freshnessDistData['15-30 days'],
                        freshnessDistData['30+ days']
                    ],
                    backgroundColor: 'rgba(13, 110, 253, 0.8)'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: { beginAtZero: true }
                }
            }
        });

        // 品質メトリクスの時系列グラフ
        const trendData = {{.TrendData}};
        const dates = trendData.map(d => d.date.split('T')[0]);
        const openCounts = trendData.map(d => d.openCount);
        const resolvedCounts = trendData.map(d => d.resolvedCount);

        new Chart(document.getElementById('trendChart'), {
            type: 'line',
            data: {
                labels: dates,
                datasets: [
                    {
                        label: '未解決',
                        data: openCounts,
                        borderColor: 'rgba(220, 53, 69, 1)',
                        backgroundColor: 'rgba(220, 53, 69, 0.2)',
                        tension: 0.4
                    },
                    {
                        label: '解決済み',
                        data: resolvedCounts,
                        borderColor: 'rgba(40, 167, 69, 1)',
                        backgroundColor: 'rgba(40, 167, 69, 0.2)',
                        tension: 0.4
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: { beginAtZero: true }
                }
            }
        });
    </script>
</body>
</html>`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
