// Package geoip 提供基于DB-IP的地理位置查询服务
package geoip

import (
	"context"
	"fmt"
	"net"

	"github.com/oschwald/maxminddb-golang"

	"github.com/weisyn/v1/internal/config/compliance"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// DBIPService 基于DB-IP数据库的地理位置查询服务
//
// 🌍 **DB-IP地理位置服务 (DB-IP Geographic Location Service)**
//
// 使用DB-IP免费数据库（Creative Commons Attribution 4.0协议）
// 提供IP地址到国家代码的高质量查询服务。
//
// 特性：
// - 587,217条记录，81%准确率
// - MMDB格式，兼容MaxMind
// - 支持自动下载和更新
// - 遵循CC Attribution 4.0协议
type DBIPService struct {
	config     *compliance.ComplianceOptions
	logger     log.Logger
	db         *maxminddb.Reader
	downloader *Downloader
}

// DBIPRecord DB-IP数据库记录结构
//
// 🗂️ **数据结构 (Data Structure)**
//
// 对应DB-IP MMDB数据库的记录格式，包含国家代码和名称信息。
type DBIPRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

// NewDBIPService 创建DB-IP地理位置服务实例
//
// 🏗️ **服务构造器 (Service Constructor)**
//
// 初始化DB-IP地理位置查询服务，加载MMDB数据库文件。
// 如果数据库文件不存在且启用了自动更新，将尝试下载数据库。
//
// 参数：
// - config: 合规配置选项，包含数据库路径和更新设置
// - logger: 日志记录器，用于记录服务状态和错误
//
// 返回：
// - complianceIfaces.GeoIPService: GeoIP服务接口实现
// - error: 初始化错误
func NewDBIPService(config *compliance.ComplianceOptions, logger log.Logger) (complianceIfaces.GeoIPService, error) {
	service := &DBIPService{
		config:     config,
		logger:     logger,
		downloader: NewDownloader(logger),
	}

	// 检查数据库路径配置
	if config.GeoIP.DatabasePath == "" {
		if logger != nil {
			logger.Debug("DB-IP数据库路径未配置，GeoIP服务将返回空结果")
		}
		return service, nil
	}

	// 尝试加载数据库
	if err := service.loadDatabase(); err != nil {
		if logger != nil {
			logger.Warnf("加载DB-IP数据库失败: %v", err)
		}

		// 如果启用自动更新，尝试下载数据库
		if config.GeoIP.AutoUpdate {
			if logger != nil {
				logger.Info("尝试自动下载DB-IP数据库...")
			}
			if err := service.downloadDatabase(); err != nil {
				if logger != nil {
					logger.Warnf("DB-IP数据库下载失败: %v", err)
					logger.Warn("DB-IP服务将以降级模式运行，GeoIP查询将返回空结果")
				}
				// 下载失败时不返回错误，继续以降级模式运行
			} else {
				// 下载成功，重新尝试加载
				if err := service.loadDatabase(); err != nil {
					if logger != nil {
						logger.Warnf("DB-IP数据库重新加载失败: %v", err)
						logger.Warn("DB-IP服务将以降级模式运行，GeoIP查询将返回空结果")
					}
					// 加载失败时也不返回错误，继续以降级模式运行
				}
			}
		} else {
			// 不自动更新时，返回警告但不失败
			if logger != nil {
				logger.Warn("DB-IP数据库加载失败且未启用自动更新，GeoIP查询将返回空结果")
			}
		}
	}

	if service.logger != nil {
		service.logger.Info("🌍 DB-IP GeoIP服务初始化完成")
		service.logger.Infof("数据库路径: %s", config.GeoIP.DatabasePath)
		service.logger.Infof("Attribution: %s", config.GeoIP.Attribution)
	}

	return service, nil
}

// GetCountryByIP 根据IP地址获取国家代码
//
// 🔍 **IP地理查询 (IP Geolocation Query)**
//
// 使用DB-IP数据库查询指定IP地址的国家代码。
// 支持IPv4和IPv6地址，返回ISO 3166-1 alpha-2国家代码。
//
// 参数：
// - ctx: 上下文，用于取消操作
// - ipAddress: IP地址字符串（IPv4或IPv6）
//
// 返回：
// - string: ISO 3166-1 alpha-2国家代码（如"US"、"CN"），空字符串表示未知
// - error: 查询错误
func (s *DBIPService) GetCountryByIP(ctx context.Context, ipAddress string) (string, error) {
	// 检查数据库是否可用
	if s.db == nil {
		if s.logger != nil {
			s.logger.Debug("DB-IP数据库未加载，返回空国家代码")
		}
		return "", nil
	}

	// 解析IP地址
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		if s.logger != nil {
			s.logger.Warnf("无效的IP地址格式: %s", ipAddress)
		}
		return "", nil
	}

	// 查询数据库
	var record DBIPRecord
	err := s.db.Lookup(ip, &record)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("查询IP %s 失败: %v", ipAddress, err)
		}
		return "", nil // 查询失败返回空，不返回错误
	}

	// 返回国家代码
	countryCode := record.Country.ISOCode
	if s.logger != nil {
		s.logger.Debugf("IP %s -> 国家: %s", ipAddress, countryCode)
	}

	return countryCode, nil
}

// UpdateDatabase 更新DB-IP数据库
//
// 🔄 **数据库更新 (Database Update)**
//
// 从DB-IP官方下载最新的免费数据库文件，解压并替换现有数据库。
// 更新完成后重新加载数据库到内存。
//
// 参数：
// - ctx: 上下文，用于取消操作
//
// 返回：
// - error: 更新错误
func (s *DBIPService) UpdateDatabase(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info("开始更新DB-IP数据库...")
	}

	// 下载新数据库
	if err := s.downloadDatabase(); err != nil {
		return err
	}

	// 重新加载数据库
	if err := s.reloadDatabase(); err != nil {
		return err
	}

	if s.logger != nil {
		s.logger.Info("DB-IP数据库更新完成")
	}

	return nil
}

// loadDatabase 加载DB-IP数据库文件到内存
func (s *DBIPService) loadDatabase() error {
	db, err := maxminddb.Open(s.config.GeoIP.DatabasePath)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

// reloadDatabase 重新加载数据库（关闭旧连接后重新打开）
func (s *DBIPService) reloadDatabase() error {
	// 关闭现有数据库连接
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}

	// 重新加载
	return s.loadDatabase()
}

// downloadDatabase 下载DB-IP数据库文件
func (s *DBIPService) downloadDatabase() error {
	if s.config.GeoIP.UpdateURL == "" {
		return fmt.Errorf("未配置数据库更新URL")
	}

	if s.logger != nil {
		s.logger.Infof("从 %s 下载DB-IP数据库...", s.config.GeoIP.UpdateURL)
	}

	// 使用下载器下载并解压数据库
	result, err := s.downloader.Download(
		context.Background(),
		s.config.GeoIP.UpdateURL,
		s.config.GeoIP.DatabasePath,
		"", // 暂不进行MD5验证，DB-IP未提供哈希值
	)

	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("DB-IP数据库下载失败: %v", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Infof("DB-IP数据库下载成功 - 文件大小: %d bytes, 耗时: %v",
			result.FileSize, result.Duration)
	}

	return nil
}
