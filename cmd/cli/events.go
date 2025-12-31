package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/transport"
)

var (
	eventsFilter map[string]string
	eventsResume string
)

// eventsCmd 事件相关命令
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "事件订阅和查询",
	Long:  "订阅区块链事件：新区块、日志、待处理交易",
}

// eventsSubscribeCmd 订阅事件
var eventsSubscribeCmd = &cobra.Command{
	Use:   "subscribe <type>",
	Short: "订阅实时事件",
	Long:  "订阅实时事件流：newHeads(新区块)、logs(事件日志)、newPendingTxs(待处理交易)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eventType := args[0]

		// 验证事件类型
		var subType transport.SubscriptionType
		switch eventType {
		case "newHeads":
			subType = transport.SubscribeNewHeads
		case "logs":
			subType = transport.SubscribeLogs
		case "newPendingTxs":
			subType = transport.SubscribeNewPendingTxs
		default:
			return fmt.Errorf("未知的事件类型: %s (可用: newHeads, logs, newPendingTxs)", eventType)
		}

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 需要WebSocket端点
		wsEndpoint := ""
		for _, ep := range profile.Endpoints {
			if ep.WS != "" {
				wsEndpoint = ep.WS
				break
			}
		}

		if wsEndpoint == "" {
			return fmt.Errorf("当前profile未配置WebSocket端点，订阅功能需要WebSocket")
		}

		// 创建WebSocket客户端
		wsClient, err := transport.NewWebSocketClient(wsEndpoint)
		if err != nil {
			return fmt.Errorf("创建WebSocket客户端失败: %w", err)
		}
		defer func() {
			if err := wsClient.Close(); err != nil {
				log.Printf("Failed to close WebSocket client: %v", err)
			}
		}()

		formatter.PrintInfo(fmt.Sprintf("正在订阅事件: %s", eventType))
		formatter.PrintInfo("按 Ctrl+C 退出")

		ctx := context.Background()

		// 构建过滤器
		filters := make(map[string]interface{})
		// 这里可以根据 eventsFilter 添加过滤条件

		// 订阅
		sub, err := wsClient.Subscribe(ctx, subType, filters, eventsResume)
		if err != nil {
			return fmt.Errorf("订阅失败: %w", err)
		}
		defer sub.Unsubscribe()

		// 监听退出信号
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// 处理事件
		for {
			select {
			case event := <-sub.Events():
				// 打印事件
				if event.Removed {
					formatter.PrintWarning(fmt.Sprintf("[重组] 事件已移除 (ReorgID: %s)", event.ReorgID))
				}

				formatter.Print(map[string]interface{}{
					"type":         event.Type,
					"data":         event.Data,
					"height":       event.Height,
					"hash":         event.Hash,
					"timestamp":    event.Timestamp,
					"removed":      event.Removed,
					"resume_token": event.ResumeToken,
				})

			case err := <-sub.Err():
				formatter.PrintError(fmt.Errorf("订阅错误: %w", err))
				return err

			case <-sigCh:
				formatter.PrintInfo("正在取消订阅...")
				return nil
			}
		}
	},
}

func init() {
	eventsCmd.AddCommand(eventsSubscribeCmd)

	// 标志
	eventsSubscribeCmd.Flags().StringToStringVar(&eventsFilter, "filter", nil, "事件过滤器 (key=value格式)")
	eventsSubscribeCmd.Flags().StringVar(&eventsResume, "resume", "", "恢复令牌 (用于断线重连)")

	// 添加到root命令
	rootCmd.AddCommand(eventsCmd)
}
