// Package output provides output formatting functionality for client commands.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// Format 输出格式
type Format string

const (
	// FormatJSON JSON格式（默认）
	FormatJSON Format = "json"
	// FormatPretty 美化JSON格式
	FormatPretty Format = "pretty"
	// FormatTable 表格格式
	FormatTable Format = "table"
	// FormatText 纯文本格式
	FormatText Format = "text"
)

// Formatter 输出格式化器
type Formatter struct {
	format     Format
	writer     io.Writer  // 数据输出（JSON/表格等）
	logWriter  io.Writer  // 日志输出（Info/Success/Error等）
	silent     bool
}

// NewFormatter 创建格式化器
func NewFormatter(format Format, writer io.Writer) *Formatter {
	if writer == nil {
		writer = os.Stdout
	}

	return &Formatter{
		format:    format,
		writer:    writer,      // 数据输出到 stdout
		logWriter: os.Stderr,   // 日志输出到 stderr（避免污染 JSON）
		silent:    false,
	}
}

// SetLogWriter 设置日志输出目标（默认 stderr）
func (f *Formatter) SetLogWriter(writer io.Writer) {
	if writer == nil {
		writer = os.Stderr
	}
	f.logWriter = writer
}

// SetSilent 设置静默模式
func (f *Formatter) SetSilent(silent bool) {
	f.silent = silent
}

// Print 打印输出
func (f *Formatter) Print(data interface{}) error {
	if f.silent {
		return nil
	}

	switch f.format {
	case FormatJSON:
		return f.printJSON(data, false)
	case FormatPretty:
		return f.printJSON(data, true)
	case FormatTable:
		return f.printTable(data)
	case FormatText:
		return f.printText(data)
	default:
		return f.printJSON(data, false)
	}
}

// printJSON 打印JSON格式
func (f *Formatter) printJSON(data interface{}, pretty bool) error {
	var output []byte
	var err error

	if pretty {
		output, err = json.MarshalIndent(data, "", "  ")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	if _, err := fmt.Fprintln(f.writer, string(output)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// printTable 打印表格格式
func (f *Formatter) printTable(data interface{}) error {
	// 使用tabwriter打印对齐的表格
	tw := tabwriter.NewWriter(f.writer, 0, 0, 2, ' ', 0)
	defer func() {
		_ = tw.Flush() // 忽略 flush 错误，因为可能已经写入部分数据
	}()

	// 根据数据类型选择表格格式
	switch v := data.(type) {
	case map[string]interface{}:
		return f.printMapTable(tw, v)
	case []interface{}:
		return f.printSliceTable(tw, v)
	case []map[string]interface{}:
		return f.printMapSliceTable(tw, v)
	default:
		// 降级到JSON
		return f.printJSON(data, true)
	}
}

// printMapTable 打印map表格
func (f *Formatter) printMapTable(tw *tabwriter.Writer, data map[string]interface{}) error {
	// 两列: Key | Value
	if _, err := fmt.Fprintln(tw, "Key\tValue"); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := fmt.Fprintln(tw, "---\t-----"); err != nil {
		return fmt.Errorf("write separator: %w", err)
	}

	for key, value := range data {
		if _, err := fmt.Fprintf(tw, "%s\t%v\n", key, formatValue(value)); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}

	return nil
}

// printSliceTable 打印slice表格
func (f *Formatter) printSliceTable(tw *tabwriter.Writer, data []interface{}) error {
	// 单列: Index | Value
	if _, err := fmt.Fprintln(tw, "#\tValue"); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := fmt.Fprintln(tw, "-\t-----"); err != nil {
		return fmt.Errorf("write separator: %w", err)
	}

	for i, value := range data {
		if _, err := fmt.Fprintf(tw, "%d\t%v\n", i, formatValue(value)); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}

	return nil
}

// printMapSliceTable 打印map slice表格
func (f *Formatter) printMapSliceTable(tw *tabwriter.Writer, data []map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	// 获取所有列
	columns := extractColumns(data)

	// 打印表头
	if _, err := fmt.Fprintln(tw, strings.Join(columns, "\t")); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := fmt.Fprintln(tw, strings.Repeat("---\t", len(columns))); err != nil {
		return fmt.Errorf("write separator: %w", err)
	}

	// 打印数据行
	for _, row := range data {
		values := make([]string, len(columns))
		for i, col := range columns {
			if val, ok := row[col]; ok {
				values[i] = formatValue(val)
			} else {
				values[i] = "-"
			}
		}
		if _, err := fmt.Fprintln(tw, strings.Join(values, "\t")); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}

	return nil
}

// printText 打印纯文本格式
func (f *Formatter) printText(data interface{}) error {
	if _, err := fmt.Fprintf(f.writer, "%v\n", data); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// PrintSuccess 打印成功消息（输出到 stderr，避免污染 JSON）
func (f *Formatter) PrintSuccess(message string) {
	if f.silent {
		return
	}
	if _, err := fmt.Fprintf(f.logWriter, "✅ %s\n", message); err != nil {
		// 忽略输出错误，因为这是辅助输出
		_ = err
	}
}

// PrintError 打印错误消息（输出到 stderr，避免污染 JSON）
func (f *Formatter) PrintError(err error) {
	if _, writeErr := fmt.Fprintf(f.logWriter, "❌ Error: %v\n", err); writeErr != nil {
		// 忽略输出错误
		_ = writeErr
	}
}

// PrintWarning 打印警告消息（输出到 stderr，避免污染 JSON）
func (f *Formatter) PrintWarning(message string) {
	if f.silent {
		return
	}
	if _, err := fmt.Fprintf(f.logWriter, "⚠️  %s\n", message); err != nil {
		// 忽略输出错误
		_ = err
	}
}

// PrintInfo 打印信息消息（输出到 stderr，避免污染 JSON）
func (f *Formatter) PrintInfo(message string) {
	if f.silent {
		return
	}
	if _, err := fmt.Fprintf(f.logWriter, "ℹ️  %s\n", message); err != nil {
		// 忽略输出错误
		_ = err
	}
}

// ===== 辅助函数 =====

// formatValue 格式化值
func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int, int64, uint, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%.2f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case time.Time:
		return v.Format(time.RFC3339)
	case nil:
		return "-"
	default:
		// 尝试JSON序列化
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

// extractColumns 提取所有列
func extractColumns(data []map[string]interface{}) []string {
	columnSet := make(map[string]bool)
	columns := make([]string, 0)

	// 收集所有列名
	for _, row := range data {
		for key := range row {
			if !columnSet[key] {
				columnSet[key] = true
				columns = append(columns, key)
			}
		}
	}

	return columns
}

// ErrorOutput 错误输出结构
type ErrorOutput struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}

// NewErrorOutput 创建错误输出
func NewErrorOutput(code string, message string, details interface{}) *ErrorOutput {
	output := &ErrorOutput{}
	output.Error.Code = code
	output.Error.Message = message
	output.Error.Details = details
	return output
}

// SuccessOutput 成功输出结构
type SuccessOutput struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// NewSuccessOutput 创建成功输出
func NewSuccessOutput(data interface{}, message string) *SuccessOutput {
	return &SuccessOutput{
		Success: true,
		Data:    data,
		Message: message,
	}
}
