package configs

import _ "embed"

// EmbeddedConfigs 嵌入的配置文件内容
type EmbeddedConfigs struct {
	Development []byte
	Testing     []byte
	Production  []byte
}

// 嵌入所有环境的配置文件（在configs目录内直接引用）
//
//go:embed development/single/config.json
var developmentConfig []byte

//go:embed testing/config.json
var testingConfig []byte

//go:embed production/config.json
var productionConfig []byte

// GetEmbeddedConfigs 获取所有嵌入的配置
func GetEmbeddedConfigs() *EmbeddedConfigs {
	return &EmbeddedConfigs{
		Development: developmentConfig,
		Testing:     testingConfig,
		Production:  productionConfig,
	}
}

// GetDevelopmentConfig 获取开发环境配置
func GetDevelopmentConfig() []byte {
	return developmentConfig
}

// GetTestingConfig 获取测试环境配置
func GetTestingConfig() []byte {
	return testingConfig
}

// GetProductionConfig 获取生产环境配置
func GetProductionConfig() []byte {
	return productionConfig
}
