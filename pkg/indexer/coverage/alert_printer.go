package coverage

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jinford/dev-rag/pkg/models"
)

// AlertPrinter はアラートを標準出力に表示します
type AlertPrinter struct {
	writer io.Writer
}

// NewAlertPrinter は新しいAlertPrinterを作成します
func NewAlertPrinter(writer io.Writer) *AlertPrinter {
	return &AlertPrinter{
		writer: writer,
	}
}

// Print はアラートを表示します
func (ap *AlertPrinter) Print(alerts []models.Alert) {
	if len(alerts) == 0 {
		fmt.Fprintln(ap.writer, "カバレッジアラートはありません。")
		return
	}

	fmt.Fprintln(ap.writer, "")
	fmt.Fprintln(ap.writer, "=== カバレッジアラート ===")
	fmt.Fprintln(ap.writer, "")

	for i, alert := range alerts {
		// Severity表示
		severityStr := ""
		switch alert.Severity {
		case models.AlertSeverityError:
			severityStr = "[エラー]"
		case models.AlertSeverityWarning:
			severityStr = "[警告]"
		}

		fmt.Fprintf(ap.writer, "%d. %s %s\n", i+1, severityStr, alert.Message)
		fmt.Fprintf(ap.writer, "   ドメイン: %s\n", alert.Domain)

		if alert.Details != nil {
			fmt.Fprintln(ap.writer, "   詳細:")
			// Details は map[string]interface{} として扱う
			if details, ok := alert.Details.(map[string]interface{}); ok {
				for key, value := range details {
					fmt.Fprintf(ap.writer, "     - %s: %v\n", key, value)
				}
			}
		}

		fmt.Fprintf(ap.writer, "   生成日時: %s\n", alert.GeneratedAt.Format(time.RFC3339))
		fmt.Fprintln(ap.writer, "")
	}

	fmt.Fprintln(ap.writer, strings.Repeat("=", 80))
}
