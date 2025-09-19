package host

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	libp2p "github.com/libp2p/go-libp2p"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"

	"github.com/libp2p/go-libp2p/core/crypto"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// 安全与身份：
// - 当前Host配置未定义细粒度SecurityConfig，使用默认安全或按常用协议启用
// - 身份选项从Base64私钥读取；若无配置则保持默认（临时身份）。

// withSecurityOptions 根据配置构建安全层选项
func withSecurityOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	if cfg == nil {
		return []libp2p.Option{libp2p.DefaultSecurity}
	}
	var opts []libp2p.Option
	if cfg.Host.Security.EnableTLS {
		opts = append(opts, libp2p.Security(tls.ID, tls.New))
	}
	if cfg.Host.Security.EnableNoise {
		opts = append(opts, libp2p.Security(noise.ID, noise.New))
	}
	if len(opts) == 0 {
		return []libp2p.Option{libp2p.DefaultSecurity}
	}
	return opts
}

// withIdentityOptions 根据配置构建身份管理选项
func withIdentityOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	if cfg == nil {
		return nil
	}
	// 1) 优先从配置中的base64私钥加载（libp2p.MarshalPrivateKey 的编码）
	if pk := strings.TrimSpace(cfg.Host.Identity.PrivateKey); pk != "" {
		if priv, ok := decodeAndUnmarshalLibp2pPriv(pk); ok {
			return []libp2p.Option{libp2p.Identity(priv)}
		}
	}

	// 2) 其次从文件加载；若文件不存在或无效，则自动生成并持久化
	keyPath := strings.TrimSpace(cfg.Host.Identity.KeyFile)
	if keyPath == "" {
		// 配置为空时报错，不应该到这里（应该在配置层设置默认值）
		return nil
	}

	if b, err := os.ReadFile(keyPath); err == nil {
		if priv, ok := decodeAndUnmarshalLibp2pPriv(strings.TrimSpace(string(b))); ok {
			return []libp2p.Option{libp2p.Identity(priv)}
		}
	}

	// 3) 生成新身份（Ed25519），并尝试持久化到 keyPath
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil
	}
	_ = persistLibp2pPrivToFile(priv, keyPath)
	return []libp2p.Option{libp2p.Identity(priv)}
}

// decodeAndUnmarshalLibp2pPriv 从base64字符串解码并反序列化libp2p私钥
func decodeAndUnmarshalLibp2pPriv(b64 string) (crypto.PrivKey, bool) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	priv, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, false
	}
	return priv, true
}

// persistLibp2pPrivToFile 将libp2p私钥(base64-encoded MarshalPrivateKey)写入文件
func persistLibp2pPrivToFile(priv crypto.PrivKey, path string) error {
	if path == "" {
		return fmt.Errorf("密钥文件路径不能为空")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString(raw)
	return os.WriteFile(path, []byte(enc), 0o600)
}
