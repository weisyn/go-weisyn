// Package geoip 提供DB-IP数据库下载和更新功能
package geoip

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Downloader DB-IP数据库下载器
//
// 📥 **数据库下载器 (Database Downloader)**
//
// 负责从DB-IP官方站点下载免费的地理位置数据库文件。
// 支持gzip压缩文件的自动解压和文件完整性验证。
//
// 特性：
// - HTTP/HTTPS下载支持
// - Gzip自动解压
// - MD5完整性验证
// - 原子性文件替换
// - 下载进度记录
type Downloader struct {
	logger log.Logger
}

// NewDownloader 创建数据库下载器实例
//
// 🏗️ **下载器构造器 (Downloader Constructor)**
//
// 参数：
// - logger: 日志记录器
//
// 返回：
// - *Downloader: 下载器实例
func NewDownloader(logger log.Logger) *Downloader {
	return &Downloader{
		logger: logger,
	}
}

// DownloadResult 下载结果
//
// 📊 **下载结果 (Download Result)**
//
// 包含下载操作的详细结果信息，用于状态跟踪和错误处理。
type DownloadResult struct {
	// 下载状态
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`

	// 文件信息
	SourceURL      string `json:"source_url"`
	TargetPath     string `json:"target_path"`
	FileSize       int64  `json:"file_size"`
	CompressedSize int64  `json:"compressed_size"`

	// 验证信息
	MD5Hash  string `json:"md5_hash"`
	Verified bool   `json:"verified"`
}

// Download 下载并解压DB-IP数据库
//
// 📥 **数据库下载 (Database Download)**
//
// 从指定URL下载gzip压缩的DB-IP数据库文件，解压后保存到目标路径。
// 支持原子性替换，确保下载过程中不会破坏现有数据库文件。
//
// 下载流程：
// 1. 创建临时文件
// 2. 下载压缩文件
// 3. 验证文件完整性
// 4. 解压到临时文件
// 5. 原子性替换目标文件
//
// 参数：
// - ctx: 上下文，支持取消操作
// - sourceURL: DB-IP数据库下载URL
// - targetPath: 目标文件路径
// - expectedMD5: 期望的MD5哈希值（可选，为空则跳过验证）
//
// 返回：
// - *DownloadResult: 下载结果详情
// - error: 下载错误
func (d *Downloader) Download(ctx context.Context, sourceURL, targetPath, expectedMD5 string) (*DownloadResult, error) {
	startTime := time.Now()
	result := &DownloadResult{
		SourceURL:  sourceURL,
		TargetPath: targetPath,
		Verified:   false,
	}

	if d.logger != nil {
		d.logger.Infof("开始下载DB-IP数据库: %s -> %s", sourceURL, targetPath)
	}

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		result.Error = fmt.Sprintf("创建目标目录失败: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// 创建临时文件
	tempFile := targetPath + ".tmp"
	defer func() {
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			if d.logger != nil {
				d.logger.Warnf("清理临时文件失败: %v", err)
			}
		}
	}()

	// 下载压缩文件
	compressedFile := tempFile + ".gz"
	defer func() {
		if err := os.Remove(compressedFile); err != nil && !os.IsNotExist(err) {
			if d.logger != nil {
				d.logger.Warnf("清理压缩文件失败: %v", err)
			}
		}
	}()

	downloadedSize, err := d.downloadFile(ctx, sourceURL, compressedFile)
	if err != nil {
		result.Error = fmt.Sprintf("下载失败: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.CompressedSize = downloadedSize

	// MD5验证（如果提供了期望哈希）
	if expectedMD5 != "" {
		actualMD5, err := d.calculateMD5(compressedFile)
		if err != nil {
			result.Error = fmt.Sprintf("MD5计算失败: %v", err)
			result.Duration = time.Since(startTime)
			return result, err
		}
		result.MD5Hash = actualMD5

		if actualMD5 != expectedMD5 {
			result.Error = fmt.Sprintf("MD5验证失败: 期望 %s, 实际 %s", expectedMD5, actualMD5)
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("文件完整性验证失败")
		}
		result.Verified = true
	}

	// 解压文件
	decompressedSize, err := d.decompressFile(compressedFile, tempFile)
	if err != nil {
		result.Error = fmt.Sprintf("解压失败: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.FileSize = decompressedSize

	// 原子性替换目标文件
	if err := os.Rename(tempFile, targetPath); err != nil {
		result.Error = fmt.Sprintf("文件替换失败: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	if d.logger != nil {
		d.logger.Infof("DB-IP数据库下载成功 - 压缩: %d bytes, 解压: %d bytes, 耗时: %v",
			result.CompressedSize, result.FileSize, result.Duration)
	}

	return result, nil
}

// downloadFile 下载文件到指定路径
func (d *Downloader) downloadFile(ctx context.Context, url, targetPath string) (int64, error) {
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	// 设置User-Agent
	req.Header.Set("User-Agent", "WES/4.0 (Blockchain Platform File System)")

	// 发送请求
	client := &http.Client{
		Timeout: 10 * time.Minute, // 10分钟超时
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			if d.logger != nil {
				d.logger.Warnf("关闭响应体失败: %v", err)
			}
		}
	}()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP错误: %d %s", resp.StatusCode, resp.Status)
	}

	// 创建输出文件
	file, err := os.Create(targetPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			if d.logger != nil {
				d.logger.Warnf("关闭文件失败: %v", err)
			}
		}
	}()

	// 复制内容
	size, err := io.Copy(file, resp.Body)
	if err != nil {
		return 0, err
	}

	if d.logger != nil {
		d.logger.Debugf("文件下载完成: %s (%d bytes)", targetPath, size)
	}

	return size, nil
}

// decompressFile 解压gzip文件
func (d *Downloader) decompressFile(compressedPath, targetPath string) (int64, error) {
	// 打开压缩文件
	compressedFile, err := os.Open(compressedPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := compressedFile.Close(); err != nil {
			if d.logger != nil {
				d.logger.Warnf("关闭压缩文件失败: %v", err)
			}
		}
	}()

	// 创建gzip读取器
	gzReader, err := gzip.NewReader(compressedFile)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := gzReader.Close(); err != nil {
			if d.logger != nil {
				d.logger.Warnf("关闭gzip读取器失败: %v", err)
			}
		}
	}()

	// 创建输出文件
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := targetFile.Close(); err != nil {
			if d.logger != nil {
				d.logger.Warnf("关闭目标文件失败: %v", err)
			}
		}
	}()

	// 解压内容
	size, err := io.Copy(targetFile, gzReader)
	if err != nil {
		return 0, err
	}

	if d.logger != nil {
		d.logger.Debugf("文件解压完成: %s (%d bytes)", targetPath, size)
	}

	return size, nil
}

// calculateMD5 计算文件MD5哈希
func (d *Downloader) calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			if d.logger != nil {
				d.logger.Warnf("关闭文件失败: %v", err)
			}
		}
	}()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
