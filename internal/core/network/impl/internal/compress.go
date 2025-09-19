package internal

// compress.go
// 能力：压缩策略适配（gzip/lz4 等），并按 NetworkConfig 的阈值进行自适应选择。
// 依赖/公共接口：
// - 若项目已有通用压缩接口（无则此处仅做第三方库适配）
// 目的：
// - 在编码层按大小阈值决定是否压缩，统一封装压缩/解压
// 非目标：
// - 不与业务耦合，不暴露到公共接口层

// Compressor 压缩器（方法框架）
type Compressor struct{}

// NewCompressor 创建压缩器
func NewCompressor() *Compressor { return &Compressor{} }

// Compress 压缩数据（方法框架）
func (c *Compressor) Compress(data []byte) ([]byte, error) { return data, nil }

// Decompress 解压数据（方法框架）
func (c *Compressor) Decompress(data []byte) ([]byte, error) { return data, nil }
