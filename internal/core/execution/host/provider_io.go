package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
)

// IOProvider IO能力提供者
//
// # 核心功能：
// - 返回数据设置：将执行结果返回给调用方
// - 沙箱文件IO：在受限目录内进行安全的文件读写操作
// - 受控日志输出：提供结构化的日志记录能力
// - 路径安全：防止目录遍历和越权访问攻击
//
// # 安全特性：
// - 沙箱隔离：所有文件操作限制在sandboxRoot目录内
// - 路径校验：严格检查相对路径，禁止绝对路径和目录遍历
// - 权限控制：文件创建使用安全的权限设置（0o644）
// - 错误处理：详细的错误信息和安全的错误传播
//
// # 设计目标：
// - 安全第一：防止恶意合约访问系统文件
// - 功能完整：支持智能合约的基本IO需求
// - 性能优化：直接文件操作，无不必要的缓存
// - 易于集成：标准的函数注入模式，灵活配置
//
// # 使用场景：
// - 智能合约的文件存储需求
// - AI模型的数据文件访问
// - 执行过程的日志记录
// - 执行结果的返回处理
type IOProvider struct {
	// setReturnData 返回数据设置函数
	// 用于将执行结果数据返回给调用方
	// nil时SetReturnData方法将返回未配置错误
	setReturnData func(data []byte) error

	// logFn 日志输出函数
	// 用于输出结构化的日志信息，支持不同级别
	// nil时Log方法将静默忽略日志（不报错）
	logFn func(level string, message string) error

	// sandboxRoot 沙箱根目录（绝对路径）
	// 所有文件操作都限制在此目录及其子目录内
	// 空字符串时文件操作将返回未配置错误
	sandboxRoot string
}

// NewIOProvider 创建IO能力提供者
//
// 返回初始化的IOProvider实例，所有功能函数都为nil，需要通过With方法配置
//
// 返回值：
//   - *IOProvider: 新创建的IO提供者实例
//
// 使用示例：
//
//	provider := NewIOProvider().
//		WithSandboxRoot("/tmp/contract_files").
//		WithLogger(loggerFunc).
//		WithSetReturnData(returnDataFunc)
//
// 设计考虑：
//   - 零配置创建，通过链式调用配置具体功能
//   - 延迟绑定，运行时检查功能可用性
func NewIOProvider() *IOProvider {
	return &IOProvider{}
}

// CapabilityDomain 返回能力领域名称
//
// 返回值：
//   - string: 固定返回"io"，标识IO能力领域
//
// 用途：
//   - 注册时的领域标识
//   - 错误信息中的领域显示
//   - 能力发现和管理
func (p *IOProvider) CapabilityDomain() string {
	return "io"
}

func (p *IOProvider) Register(r execiface.HostCapabilityRegistry) error {
	if err := r.RegisterProvider(p); err != nil {
		return fmt.Errorf("register io provider failed: %w", err)
	}
	return nil
}

// WithSetReturnData 注入返回负载处理函数
func (p *IOProvider) WithSetReturnData(fn func(data []byte) error) *IOProvider {
	p.setReturnData = fn
	return p
}

// WithLogger 注入日志输出函数
func (p *IOProvider) WithLogger(fn func(level string, message string) error) *IOProvider {
	p.logFn = fn
	return p
}

// WithSandboxRoot 配置允许访问的根目录（绝对路径）
func (p *IOProvider) WithSandboxRoot(root string) *IOProvider {
	p.sandboxRoot = root
	return p
}

// ReadFile 在沙箱内读取文件
func (p *IOProvider) ReadFile(path string) ([]byte, error) {
	if p.sandboxRoot == "" {
		return nil, errors.New("sandbox root not configured")
	}
	abs, err := p.resolveSandboxPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("read file failed: %w", err)
	}
	return data, nil
}

// WriteFile 在沙箱内写入文件（创建父目录）
func (p *IOProvider) WriteFile(path string, data []byte) error {
	if p.sandboxRoot == "" {
		return errors.New("sandbox root not configured")
	}
	abs, err := p.resolveSandboxPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("mkdir parent failed: %w", err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}
	return nil
}

// Log 受控日志输出
func (p *IOProvider) Log(level string, message string) error {
	if p.logFn == nil {
		return nil
	}
	lvl := strings.ToLower(strings.TrimSpace(level))
	if lvl == "" {
		lvl = "info"
	}
	return p.logFn(lvl, message)
}

// SetReturnData 设置返回负载（供引擎归档结果时调用）
func (p *IOProvider) SetReturnData(data []byte) error {
	if p.setReturnData == nil {
		return fmt.Errorf("setReturnData not configured")
	}
	return p.setReturnData(data)
}

// resolveSandboxPath 校验并解析目标路径到沙箱内的绝对路径
func (p *IOProvider) resolveSandboxPath(rel string) (string, error) {
	if rel == "" {
		return "", errors.New("path is empty")
	}
	// 仅允许相对路径，禁止以绝对路径或根起始
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed: %s", rel)
	}
	clean := filepath.Clean(rel)
	joined := filepath.Join(p.sandboxRoot, clean)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve path failed: %w", err)
	}
	// 防止目录遍历：必须以 sandboxRoot 为前缀
	rootAbs, err := filepath.Abs(p.sandboxRoot)
	if err != nil {
		return "", fmt.Errorf("resolve root failed: %w", err)
	}
	// 统一分隔符大小写
	a := filepath.ToSlash(abs)
	r := filepath.ToSlash(rootAbs)
	if !strings.HasPrefix(a+"/", r+"/") && a != r {
		return "", fmt.Errorf("path escapes sandbox: %s", rel)
	}
	return abs, nil
}
