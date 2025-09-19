package utils

import (
	"net/http"
	"path/filepath"
	"strings"
)

// DetectMimeType 智能检测文件的MIME类型
//
// 使用多种方法综合检测：
// 1. Go标准库 http.DetectContentType (基于文件头魔数)
// 2. 文件扩展名映射
// 3. 区块链特殊文件类型检测
//
// 参数：
//
//	data: 文件内容字节
//	fileName: 文件名（可选，用于扩展名检测）
//
// 返回：
//
//	string: 检测到的MIME类型
func DetectMimeType(data []byte, fileName ...string) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}

	// 🎯 方法1：使用Go标准库的智能检测（基于文件头魔数）
	detectedType := http.DetectContentType(data)

	// 🎯 方法2：如果提供了文件名，使用扩展名检测
	var extType string
	if len(fileName) > 0 && fileName[0] != "" {
		ext := strings.ToLower(filepath.Ext(fileName[0]))
		extType = getMimeTypeByExtension(ext)
	}

	// 🎯 方法3：区块链特殊文件类型检测
	var specialType string
	if isWASMBytecode(data) {
		specialType = "application/wasm"
	} else if isONNXModel(data) {
		specialType = "application/onnx"
	}

	// 🔧 智能选择最准确的结果
	if specialType != "" {
		return specialType
	}

	if extType != "" && extType != "application/octet-stream" {
		return extType
	}

	return detectedType
}

// getMimeTypeByExtension 根据文件扩展名获取MIME类型
func getMimeTypeByExtension(ext string) string {
	// 常见文件类型映射
	mimeMap := map[string]string{
		".wasm": "application/wasm",
		".onnx": "application/onnx",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".ts":   "application/typescript",
		".go":   "text/x-go",
		".py":   "text/x-python",
		".java": "text/x-java-source",
		".c":    "text/x-c",
		".cpp":  "text/x-c++",
		".h":    "text/x-c",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".7z":   "application/x-7z-compressed",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
		".bz2":  "application/x-bzip2",
		".xz":   "application/x-xz",
	}

	if mimeType, exists := mimeMap[ext]; exists {
		return mimeType
	}

	return "application/octet-stream"
}

// isWASMBytecode 检查是否为WASM字节码
func isWASMBytecode(data []byte) bool {
	// WASM魔数检查：0x00 0x61 0x73 0x6D
	wasmMagic := []byte{0x00, 0x61, 0x73, 0x6D}

	if len(data) < 4 {
		return false
	}

	// 比较前4字节是否匹配WASM魔数
	for i := 0; i < 4; i++ {
		if data[i] != wasmMagic[i] {
			return false
		}
	}
	return true
}

// isONNXModel 检查是否为ONNX模型
func isONNXModel(data []byte) bool {
	if len(data) < 8 {
		return false
	}

	// ONNX模型通常包含特定标识
	checkLen := len(data)
	if checkLen > 100 {
		checkLen = 100
	}
	dataStr := string(data[:checkLen])
	return strings.Contains(dataStr, "onnx") ||
		strings.Contains(dataStr, "ONNX") ||
		strings.Contains(dataStr, "GraphProto")
}

// GetFileExtension 从文件名获取扩展名
func GetFileExtension(fileName string) string {
	return strings.ToLower(filepath.Ext(fileName))
}

// IsExecutableFile 检查文件是否为可执行类型
func IsExecutableFile(mimeType string) bool {
	executableTypes := []string{
		"application/wasm",
		"application/onnx",
		"application/x-executable",
		"application/x-elf",
	}

	for _, execType := range executableTypes {
		if strings.HasPrefix(mimeType, execType) {
			return true
		}
	}
	return false
}

