package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/weisyn/v1/pkg/interfaces/config"
)

// Server gRPC服务器
// 🚧 阶段性骨架实现：启用反射，支持 grpcurl 调试
//
// 功能：
// - 启用 gRPC 反射（支持 grpcurl 动态探测）
// - 提供生命周期管理（Start/Stop）
// - 预留服务注册接口
//
// 后续扩展：
// - 注册 BlockchainService
// - 注册 TransactionService
// - 注册 NodeService
type Server struct {
	logger     *zap.Logger
	config     config.Provider
	grpcServer *grpc.Server
	listener   net.Listener
	actualAddr string
}

// NewServer 创建 gRPC 服务器
//
// 参数：
//   - logger: 日志记录器
//   - cfg: 配置提供者
//
// 返回：gRPC 服务器实例（已启用反射）
func NewServer(
	logger *zap.Logger,
	cfg config.Provider,
) *Server {
	// 创建 gRPC 服务器（使用默认选项）
	grpcServer := grpc.NewServer()

	// 启用反射（支持 grpcurl 探测）
	reflection.Register(grpcServer)

	logger.Info("gRPC server initialized with reflection enabled",
		zap.String("status", "skeleton"))

	return &Server{
		logger:     logger,
		config:     cfg,
		grpcServer: grpcServer,
	}
}

// Start 启动 gRPC 服务器
func (s *Server) Start(ctx context.Context) error {
	host := s.config.GetAPI().GRPC.Host
	port := s.config.GetAPI().GRPC.Port
	addr := fmt.Sprintf("%s:%d", host, port)

	// 创建监听器（端口占用时自动递增重试，避免启动直接失败）
	const maxTries = 20
	allowAutoSelect := s.config != nil && s.config.GetEnvironment() != "prod"
	var (
		listener net.Listener
		err      error
	)
	for i := 0; i < maxTries; i++ {
		// prod 环境 fail-fast：只尝试配置端口一次，避免“静默变更服务端口”
		if i > 0 && !allowAutoSelect {
			break
		}
		addr = fmt.Sprintf("%s:%d", host, port+i)
		listener, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
		// 仅对端口占用做重试，其它错误直接失败
		if !errors.Is(err, syscall.EADDRINUSE) {
			s.logger.Error("Failed to create gRPC listener",
				zap.String("addr", addr),
				zap.Error(err))
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
		if allowAutoSelect && s.logger != nil {
			s.logger.Warn("gRPC port already in use; auto-selecting another port (non-prod only)",
				zap.String("configured_addr", fmt.Sprintf("%s:%d", host, port)),
				zap.String("attempt_addr", addr),
				zap.Int("attempt", i+1),
				zap.Int("max_tries", maxTries))
		}
	}
	if err != nil {
		// 全部尝试失败
		s.logger.Error("Failed to create gRPC listener",
			zap.String("addr", addr),
			zap.Error(err))
		if errors.Is(err, syscall.EADDRINUSE) {
			// prod 给出明确提示
			return fmt.Errorf("port already in use: %s (hint: set api.grpc_port in config)", fmt.Sprintf("%s:%d", host, port))
		}
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener
	s.actualAddr = listener.Addr().String()

	s.logger.Info("Starting gRPC server",
		zap.String("address", s.actualAddr))

	// 在后台启动 gRPC 服务器
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			s.logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	s.logger.Info("✅ gRPC server started (reflection enabled)",
		zap.String("address", s.actualAddr),
		zap.String("hint", "Use grpcurl for testing: grpcurl -plaintext "+s.actualAddr+" list"))

	return nil
}

// Address 返回实际监听地址（用于在端口冲突自动切换时打印真实端口）。
func (s *Server) Address() string {
	if s == nil {
		return ""
	}
	return s.actualAddr
}

// Stop 停止 gRPC 服务器
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping gRPC server...")

	// 优雅停止
	s.grpcServer.GracefulStop()

	// 关闭监听器
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Warn("Failed to close gRPC listener", zap.Error(err))
		}
	}

	s.logger.Info("✅ gRPC server stopped")
	return nil
}

// RegisterService 注册 gRPC 服务
// 🚧 预留接口，待后续实现具体服务时使用
//
// 使用示例：
//
//	server.RegisterService(func(s *grpc.Server) {
//	    pb.RegisterBlockchainServiceServer(s, blockchainSvc)
//	})
func (s *Server) RegisterService(register func(*grpc.Server)) {
	register(s.grpcServer)
	s.logger.Info("gRPC service registered")
}
