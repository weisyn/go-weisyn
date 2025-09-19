// Package resource - MIME类型检测工具
//
// 🎯 **MIME类型检测器 (MIME Type Detector)**
//
// 本文件提供静态资源的MIME类型检测功能：
// - 基于文件头魔数的检测
// - 基于文件扩展名的检测
// - 基于内容特征的检测
//
// 🏗️ **设计原则**：
// - 多层检测：魔数 -> 扩展名 -> 内容特征
// - 高准确性：支持主流文件格式的精确识别
// - 扩展性强：易于添加新的文件类型支持
package resource

import (
	"bytes"
	"mime"
	"path/filepath"
	"strings"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// MimeDetector MIME类型检测器
type MimeDetector struct {
	logger log.Logger
}

// NewMimeDetector 创建MIME检测器实例
func NewMimeDetector(logger log.Logger) *MimeDetector {
	return &MimeDetector{
		logger: logger,
	}
}

// DetectResourceMimeType 检测资源的MIME类型
//
// 🎯 **多层检测策略**：
// 1. 基于文件头魔数检测（最准确）
// 2. 基于文件扩展名检测（常用格式）
// 3. 基于内容特征检测（特殊情况）
//
// 参数：
//   - resourceData: 资源数据（文件头部分）
//   - filePath: 文件路径（用于扩展名检测）
//
// 返回：
//   - string: 检测到的MIME类型
func (md *MimeDetector) DetectResourceMimeType(resourceData []byte, filePath string) string {
	// 🔍 第一层：基于文件头魔数检测（最准确的方法）
	mimeType := md.DetectMimeByMagicNumbers(resourceData)
	if mimeType != "application/octet-stream" {
		return mimeType
	}

	// 🔍 第二层：基于文件扩展名检测
	if filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		if extMimeType := mime.TypeByExtension(ext); extMimeType != "" {
			return extMimeType
		}
	}

	// 🔍 第三层：基于内容特征检测
	mimeType = md.DetectMimeByContent(resourceData)
	if mimeType != "application/octet-stream" {
		return mimeType
	}

	return "application/octet-stream" // 默认二进制类型
}

// DetectMimeByMagicNumbers 基于文件头魔数检测MIME类型
func (md *MimeDetector) DetectMimeByMagicNumbers(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}

	// 🔍 图像格式检测
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg"
	}
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif"
	}
	if bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}

	// 🔍 文档格式检测
	if bytes.HasPrefix(data, []byte{0x25, 0x50, 0x44, 0x46}) { // %PDF
		return "application/pdf"
	}
	if bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x03, 0x04}) || bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x05, 0x06}) {
		// ZIP格式（包括Office文档）
		return md.detectOfficeDocument(data)
	}
	if bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return "application/msword" // 老版本Office文档
	}

	// 🔍 音视频格式检测
	if bytes.HasPrefix(data, []byte("ftyp")) && len(data) >= 8 {
		return "video/mp4"
	}
	if bytes.HasPrefix(data, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return "video/webm"
	}
	if bytes.HasPrefix(data, []byte("ID3")) || bytes.HasPrefix(data, []byte{0xFF, 0xFB}) {
		return "audio/mpeg"
	}

	// 🔍 压缩格式检测
	if bytes.HasPrefix(data, []byte{0x1F, 0x8B}) {
		return "application/gzip"
	}
	if bytes.HasPrefix(data, []byte("7z")) {
		return "application/x-7z-compressed"
	}
	if bytes.HasPrefix(data, []byte("Rar!")) {
		return "application/x-rar-compressed"
	}

	// 🔍 代码文件检测
	if md.isTextContent(data) {
		return "text/plain"
	}

	return "application/octet-stream"
}

// DetectMimeByContent 基于内容特征检测MIME类型
func (md *MimeDetector) DetectMimeByContent(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}

	// 检查是否为文本内容
	if md.isTextContent(data) {
		// 进一步检查具体的文本类型
		content := string(data)
		if strings.Contains(content, "<?xml") {
			return "application/xml"
		}
		if strings.Contains(content, "{") && strings.Contains(content, "}") {
			return "application/json"
		}
		if strings.Contains(content, "<!DOCTYPE html") || strings.Contains(content, "<html") {
			return "text/html"
		}
		return "text/plain"
	}

	return "application/octet-stream"
}

// detectOfficeDocument 检测Office文档类型
func (md *MimeDetector) detectOfficeDocument(data []byte) string {
	// 简化实现：ZIP格式的文档默认为通用Office文档
	// 实际实现中可以通过解析ZIP内容来精确识别
	return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}

// isTextContent 检查是否为文本内容
func (md *MimeDetector) isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// 检查前512字节中是否包含不可打印字符
	checkSize := 512
	if len(data) < checkSize {
		checkSize = len(data)
	}

	nullCount := 0
	for i := 0; i < checkSize; i++ {
		b := data[i]
		// 检查是否为控制字符（除了常见的换行符等）
		if b == 0 {
			nullCount++
		} else if b < 32 && b != 9 && b != 10 && b != 13 {
			// 如果包含太多控制字符，可能不是文本文件
			if nullCount > checkSize/100 { // 超过1%的null字符
				return false
			}
		}
	}

	return nullCount <= checkSize/100 // null字符不超过1%
}
