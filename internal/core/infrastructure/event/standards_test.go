package event

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEventName(t *testing.T) {
	t.Run("有效事件名", func(t *testing.T) {
		validNames := []string{
			"blockchain.block.produced",
			"mempool.tx.added",
			"consensus.round.completed",
			"network.peer.connected",
			"system.startup.completed",
			"blockchain.block.confirmed.final",
			"user.profile.updated.email",
			"order.payment.processed.success",
			"file.upload.completed.with_metadata",
		}

		for _, name := range validNames {
			err := ValidateEventName(name)
			assert.NoError(t, err, "Event name %s should be valid", name)
		}
	})

	t.Run("无效事件名", func(t *testing.T) {
		invalidCases := []struct {
			name         string
			expectedCode string
			description  string
		}{
			{"", "EMPTY_EVENT_NAME", "空事件名"},
			{"blockchain", "INSUFFICIENT_EVENT_NAME_PARTS", "部分不足"},
			{"blockchain.block", "INSUFFICIENT_EVENT_NAME_PARTS", "部分不足"},
			{"Blockchain.block.produced", "INVALID_EVENT_NAME_FORMAT", "大写字母"},
			{"blockchain.Block.produced", "INVALID_EVENT_NAME_FORMAT", "大写实体"},
			{"blockchain.block.Produced", "INVALID_EVENT_NAME_FORMAT", "大写动作"},
			{"blockchain-test.block.produced", "INVALID_EVENT_NAME_FORMAT", "域名包含连字符"},
			{"123blockchain.block.produced", "INVALID_EVENT_NAME_FORMAT", "域名数字开头"},
			{"blockchain.block.produced.detail.extra.more.parts.exceed.limit", "TOO_MANY_EVENT_NAME_PARTS", "部分过多"},
			{strings.Repeat("a", 201), "EVENT_NAME_TOO_LONG", "名称过长"},
		}

		for _, tc := range invalidCases {
			err := ValidateEventName(tc.name)
			assert.Error(t, err, "Event name %s should be invalid: %s", tc.name, tc.description)

			if validationErr, ok := err.(*ValidationError); ok {
				assert.Equal(t, tc.expectedCode, validationErr.Code,
					"Error code mismatch for %s", tc.description)
			}
		}
	})

	t.Run("边界情况", func(t *testing.T) {
		// 最短有效名称
		shortName := "abc.def.ghi"
		assert.NoError(t, ValidateEventName(shortName))

		// 最长域名
		longDomain := strings.Repeat("a", 32)
		longName := longDomain + ".entity.action"
		assert.NoError(t, ValidateEventName(longName))

		// 超长域名
		tooLongDomain := strings.Repeat("a", 33)
		tooLongName := tooLongDomain + ".entity.action"
		assert.Error(t, ValidateEventName(tooLongName))
	})
}

func TestValidateDomainName(t *testing.T) {
	t.Run("有效域名", func(t *testing.T) {
		validDomains := []string{
			"blockchain",
			"mempool",
			"consensus",
			"network",
			"user_service",
			"payment_gateway",
			"file_storage",
			"abc",                   // 最短
			strings.Repeat("a", 32), // 最长
		}

		for _, domain := range validDomains {
			err := ValidateDomainName(domain)
			assert.NoError(t, err, "Domain %s should be valid", domain)
		}
	})

	t.Run("无效域名", func(t *testing.T) {
		invalidCases := []struct {
			domain       string
			expectedCode string
		}{
			{"", "EMPTY_DOMAIN_NAME"},
			{"Blockchain", "INVALID_DOMAIN_NAME_FORMAT"},
			{"blockchain-service", "INVALID_DOMAIN_NAME_FORMAT"},
			{"123blockchain", "INVALID_DOMAIN_NAME_FORMAT"},
			{"ab", "INVALID_DOMAIN_NAME_FORMAT"},              // 太短
			{strings.Repeat("a", 33), "DOMAIN_NAME_TOO_LONG"}, // 太长
		}

		for _, tc := range invalidCases {
			err := ValidateDomainName(tc.domain)
			assert.Error(t, err)

			if validationErr, ok := err.(*ValidationError); ok {
				assert.Equal(t, tc.expectedCode, validationErr.Code)
			}
		}
	})
}

func TestValidateEntityName(t *testing.T) {
	t.Run("有效实体名", func(t *testing.T) {
		validEntities := []string{
			"block",
			"transaction",
			"user",
			"order",
			"payment",
			"file",
			"user_profile",
			"order_item",
			"a",                     // 最短
			strings.Repeat("a", 32), // 最长
		}

		for _, entity := range validEntities {
			err := ValidateEntityName(entity)
			assert.NoError(t, err, "Entity %s should be valid", entity)
		}
	})

	t.Run("无效实体名", func(t *testing.T) {
		invalidCases := []struct {
			entity       string
			expectedCode string
		}{
			{"", "EMPTY_ENTITY_NAME"},
			{"Block", "INVALID_ENTITY_NAME_FORMAT"},
			{"user-profile", "INVALID_ENTITY_NAME_FORMAT"},
			{"123user", "INVALID_ENTITY_NAME_FORMAT"},
			{strings.Repeat("a", 33), "ENTITY_NAME_TOO_LONG"},
		}

		for _, tc := range invalidCases {
			err := ValidateEntityName(tc.entity)
			assert.Error(t, err)

			if validationErr, ok := err.(*ValidationError); ok {
				assert.Equal(t, tc.expectedCode, validationErr.Code)
			}
		}
	})
}

func TestValidateActionName(t *testing.T) {
	t.Run("有效动作名", func(t *testing.T) {
		validActions := []string{
			"created",
			"updated",
			"deleted",
			"processed",
			"completed",
			"failed",
			"upload_started",
			"payment_processed",
			"a",                     // 最短
			strings.Repeat("a", 32), // 最长
		}

		for _, action := range validActions {
			err := ValidateActionName(action)
			assert.NoError(t, err, "Action %s should be valid", action)
		}
	})

	t.Run("无效动作名", func(t *testing.T) {
		invalidCases := []struct {
			action       string
			expectedCode string
		}{
			{"", "EMPTY_ACTION_NAME"},
			{"Created", "INVALID_ACTION_NAME_FORMAT"},
			{"user-created", "INVALID_ACTION_NAME_FORMAT"},
			{"123created", "INVALID_ACTION_NAME_FORMAT"},
			{strings.Repeat("a", 33), "ACTION_NAME_TOO_LONG"},
		}

		for _, tc := range invalidCases {
			err := ValidateActionName(tc.action)
			assert.Error(t, err)

			if validationErr, ok := err.(*ValidationError); ok {
				assert.Equal(t, tc.expectedCode, validationErr.Code)
			}
		}
	})
}

func TestValidateEventData(t *testing.T) {
	t.Run("有效事件数据", func(t *testing.T) {
		validData := []interface{}{
			nil,
			"simple string",
			123,
			map[string]interface{}{"key": "value"},
			[]string{"item1", "item2"},
			struct{ Name string }{"test"},
		}

		for _, data := range validData {
			err := ValidateEventData(data)
			assert.NoError(t, err, "Data should be valid: %+v", data)
		}
	})

	t.Run("过大的事件数据", func(t *testing.T) {
		// 创建一个大数据
		largeData := strings.Repeat("a", MaxEventDataSize+1)
		err := ValidateEventData(largeData)
		assert.Error(t, err)

		if validationErr, ok := err.(*ValidationError); ok {
			assert.Equal(t, "EVENT_DATA_TOO_LARGE", validationErr.Code)
		}
	})
}

func TestEventNameBuilder(t *testing.T) {
	t.Run("创建构建器", func(t *testing.T) {
		builder, err := NewEventNameBuilder("blockchain")
		assert.NoError(t, err)
		assert.NotNil(t, builder)
		assert.Equal(t, "blockchain", builder.GetDomain())
	})

	t.Run("无效域名创建构建器应该失败", func(t *testing.T) {
		_, err := NewEventNameBuilder("Invalid-Domain")
		assert.Error(t, err)
	})

	t.Run("构建基础事件名", func(t *testing.T) {
		builder, err := NewEventNameBuilder("blockchain")
		require.NoError(t, err)

		eventName, err := builder.Build("block", "produced")
		assert.NoError(t, err)
		assert.Equal(t, "blockchain.block.produced", eventName)
	})

	t.Run("构建带详情的事件名", func(t *testing.T) {
		builder, err := NewEventNameBuilder("payment")
		require.NoError(t, err)

		eventName, err := builder.BuildWithDetail("order", "processed", "success")
		assert.NoError(t, err)
		assert.Equal(t, "payment.order.processed.success", eventName)

		// 多个详情
		eventName, err = builder.BuildWithDetail("transaction", "failed", "timeout", "retry")
		assert.NoError(t, err)
		assert.Equal(t, "payment.transaction.failed.timeout.retry", eventName)
	})

	t.Run("构建无效名称应该失败", func(t *testing.T) {
		builder, err := NewEventNameBuilder("blockchain")
		require.NoError(t, err)

		// 无效实体名
		_, err = builder.Build("Invalid-Entity", "created")
		assert.Error(t, err)

		// 无效动作名
		_, err = builder.Build("entity", "Invalid-Action")
		assert.Error(t, err)

		// 无效详情
		_, err = builder.BuildWithDetail("entity", "action", "Invalid-Detail")
		assert.Error(t, err)
	})
}

func TestEventNameParser(t *testing.T) {
	parser := NewEventNameParser()

	t.Run("解析标准事件名", func(t *testing.T) {
		eventName := "blockchain.block.produced"
		parsed, err := parser.Parse(eventName)

		assert.NoError(t, err)
		require.NotNil(t, parsed)
		assert.Equal(t, "blockchain.block.produced", parsed.FullName)
		assert.Equal(t, "blockchain", parsed.Domain)
		assert.Equal(t, "block", parsed.Entity)
		assert.Equal(t, "produced", parsed.Action)
		assert.False(t, parsed.HasDetails())
		assert.Equal(t, "blockchain.block.produced", parsed.GetBaseName())
	})

	t.Run("解析带详情的事件名", func(t *testing.T) {
		eventName := "payment.order.processed.success.confirmed"
		parsed, err := parser.Parse(eventName)

		assert.NoError(t, err)
		require.NotNil(t, parsed)
		assert.Equal(t, "payment", parsed.Domain)
		assert.Equal(t, "order", parsed.Entity)
		assert.Equal(t, "processed", parsed.Action)
		assert.True(t, parsed.HasDetails())
		assert.Equal(t, []string{"success", "confirmed"}, parsed.Details)
		assert.Equal(t, "success.confirmed", parsed.GetDetailString())
		assert.Equal(t, "payment.order.processed", parsed.GetBaseName())
	})

	t.Run("解析无效事件名应该失败", func(t *testing.T) {
		invalidNames := []string{
			"",
			"invalid",
			"invalid.name",
			"Invalid.Entity.Action",
		}

		for _, name := range invalidNames {
			_, err := parser.Parse(name)
			assert.Error(t, err, "Should fail to parse: %s", name)
		}
	})

	t.Run("提取函数", func(t *testing.T) {
		eventName := "user.profile.updated.email"

		assert.Equal(t, "user", parser.ExtractDomain(eventName))
		assert.Equal(t, "profile", parser.ExtractEntity(eventName))
		assert.Equal(t, "updated", parser.ExtractAction(eventName))
	})

	t.Run("提取函数边界情况", func(t *testing.T) {
		// 空字符串
		assert.Equal(t, "", parser.ExtractDomain(""))
		assert.Equal(t, "", parser.ExtractEntity(""))
		assert.Equal(t, "", parser.ExtractAction(""))

		// 单个部分
		assert.Equal(t, "domain", parser.ExtractDomain("domain"))
		assert.Equal(t, "", parser.ExtractEntity("domain"))
		assert.Equal(t, "", parser.ExtractAction("domain"))

		// 两个部分
		assert.Equal(t, "domain", parser.ExtractDomain("domain.entity"))
		assert.Equal(t, "entity", parser.ExtractEntity("domain.entity"))
		assert.Equal(t, "", parser.ExtractAction("domain.entity"))
	})
}

func TestEventMetadata(t *testing.T) {
	t.Run("创建事件元数据", func(t *testing.T) {
		eventName := "blockchain.block.produced"
		source := "blockchain-node-1"

		metadata, err := NewEventMetadata(eventName, source)
		assert.NoError(t, err)
		require.NotNil(t, metadata)

		assert.Equal(t, eventName, metadata.Name)
		assert.Equal(t, source, metadata.Source)
		assert.Equal(t, "blockchain", metadata.Domain)
		assert.Equal(t, "block", metadata.Entity)
		assert.Equal(t, "produced", metadata.Action)
		assert.Equal(t, "1.0", metadata.Version)
		assert.Equal(t, PriorityNormal, metadata.Priority)
		assert.Equal(t, int64(3600), metadata.TTL)
		assert.True(t, metadata.Retryable)
		assert.False(t, metadata.Idempotent)
		assert.NotEmpty(t, metadata.ID)
		assert.False(t, metadata.Timestamp.IsZero())
		assert.NotNil(t, metadata.Tags)
		assert.NotNil(t, metadata.Labels)
		assert.NotNil(t, metadata.Context)
		assert.NotNil(t, metadata.Properties)
	})

	t.Run("无效事件名创建元数据应该失败", func(t *testing.T) {
		_, err := NewEventMetadata("invalid-event-name", "source")
		assert.Error(t, err)
	})
}

func TestValidationError(t *testing.T) {
	t.Run("创建和使用验证错误", func(t *testing.T) {
		err := NewValidationError("test message", "TEST_CODE")

		assert.Equal(t, "test message", err.Message)
		assert.Equal(t, "TEST_CODE", err.Code)
		assert.Equal(t, "[TEST_CODE] test message", err.Error())
		assert.True(t, IsValidationError(err))
	})

	t.Run("检查非验证错误", func(t *testing.T) {
		err := assert.AnError
		assert.False(t, IsValidationError(err))
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("快速提取函数", func(t *testing.T) {
		eventName := "user.profile.updated.email"

		assert.Equal(t, "user", ExtractDomainFromEventName(eventName))
		assert.Equal(t, "profile", ExtractEntityFromEventName(eventName))
		assert.Equal(t, "updated", ExtractActionFromEventName(eventName))
	})

	t.Run("标准化事件名", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"  User.Profile.Updated  ", "user.profile.updated"},
			{"BLOCKCHAIN.BLOCK.PRODUCED", "blockchain.block.produced"},
			{"blockchain.block.produced", "blockchain.block.produced"},
			{"  ", ""},
		}

		for _, tc := range testCases {
			result := NormalizeEventName(tc.input)
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("检查标准事件名", func(t *testing.T) {
		assert.True(t, IsStandardEventName("blockchain.block.produced"))
		assert.False(t, IsStandardEventName("Invalid.Event.Name"))
		assert.False(t, IsStandardEventName(""))
	})

	t.Run("获取事件名称模式", func(t *testing.T) {
		pattern := GetEventNamePattern()
		assert.Equal(t, EventNamePattern, pattern)
	})

	t.Run("获取事件名称信息", func(t *testing.T) {
		eventName := "blockchain.block.produced.final"
		info := GetEventNameInfo(eventName)

		assert.True(t, info["valid"].(bool))
		assert.Equal(t, len(eventName), info["length"])
		assert.Equal(t, MaxEventNameLength, info["max_length"])
		assert.Equal(t, 4, info["parts"])
		assert.Equal(t, "blockchain", info["domain"])
		assert.Equal(t, "block", info["entity"])
		assert.Equal(t, "produced", info["action"])
		assert.Equal(t, []string{"final"}, info["details"])
	})

	t.Run("获取无效事件名称信息", func(t *testing.T) {
		eventName := "invalid"
		info := GetEventNameInfo(eventName)

		assert.False(t, info["valid"].(bool))
		assert.Equal(t, len(eventName), info["length"])
	})
}

func TestConstants(t *testing.T) {
	t.Run("验证常量值", func(t *testing.T) {
		assert.Equal(t, 200, MaxEventNameLength)
		assert.Equal(t, 32, MaxDomainNameLength)
		assert.Equal(t, 32, MaxEntityNameLength)
		assert.Equal(t, 32, MaxActionNameLength)
		assert.Equal(t, 1024*1024, MaxEventDataSize)
		assert.Equal(t, 3, MinEventNameParts)
		assert.Equal(t, 8, MaxEventNameParts)
		assert.Equal(t, ".", EventNameSeparator)
		assert.Equal(t, "v", VersionSeparator)
	})

	t.Run("正则表达式编译", func(t *testing.T) {
		assert.NotNil(t, eventNameRegex)
		assert.NotNil(t, domainNameRegex)
		assert.NotNil(t, entityNameRegex)
		assert.NotNil(t, actionNameRegex)
	})
}

func TestParsedEventName(t *testing.T) {
	t.Run("ParsedEventName方法", func(t *testing.T) {
		parsed := &ParsedEventName{
			FullName: "blockchain.block.produced.final",
			Domain:   "blockchain",
			Entity:   "block",
			Action:   "produced",
			Details:  []string{"final"},
		}

		assert.Equal(t, "blockchain.block.produced.final", parsed.String())
		assert.Equal(t, "blockchain.block.produced", parsed.GetBaseName())
		assert.True(t, parsed.HasDetails())
		assert.Equal(t, "final", parsed.GetDetailString())
	})

	t.Run("无详情的ParsedEventName", func(t *testing.T) {
		parsed := &ParsedEventName{
			FullName: "blockchain.block.produced",
			Domain:   "blockchain",
			Entity:   "block",
			Action:   "produced",
			Details:  []string{},
		}

		assert.False(t, parsed.HasDetails())
		assert.Equal(t, "", parsed.GetDetailString())
	})

	t.Run("多个详情的ParsedEventName", func(t *testing.T) {
		parsed := &ParsedEventName{
			FullName: "payment.order.processed.success.confirmed",
			Domain:   "payment",
			Entity:   "order",
			Action:   "processed",
			Details:  []string{"success", "confirmed"},
		}

		assert.True(t, parsed.HasDetails())
		assert.Equal(t, "success.confirmed", parsed.GetDetailString())
	})
}

// BenchmarkValidateEventName 性能基准测试
func BenchmarkValidateEventName(b *testing.B) {
	eventName := "blockchain.block.produced"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateEventName(eventName)
	}
}

// BenchmarkEventNameBuilder 性能基准测试
func BenchmarkEventNameBuilder(b *testing.B) {
	builder, _ := NewEventNameBuilder("blockchain")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Build("block", "produced")
	}
}

// BenchmarkEventNameParser 性能基准测试
func BenchmarkEventNameParser(b *testing.B) {
	parser := NewEventNameParser()
	eventName := "blockchain.block.produced.final"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(eventName)
	}
}

// BenchmarkExtractDomain 性能基准测试
func BenchmarkExtractDomain(b *testing.B) {
	eventName := "blockchain.block.produced.final"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractDomainFromEventName(eventName)
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("并发验证事件名", func(t *testing.T) {
		eventNames := []string{
			"blockchain.block.produced",
			"mempool.tx.added",
			"consensus.round.completed",
			"network.peer.connected",
		}

		// 启动多个goroutine并发验证
		done := make(chan bool, len(eventNames))
		for _, name := range eventNames {
			go func(eventName string) {
				for i := 0; i < 1000; i++ {
					ValidateEventName(eventName)
				}
				done <- true
			}(name)
		}

		// 等待所有goroutine完成
		for i := 0; i < len(eventNames); i++ {
			<-done
		}
	})

	t.Run("并发使用构建器", func(t *testing.T) {
		builder, err := NewEventNameBuilder("test")
		require.NoError(t, err)

		entities := []string{"entity1", "entity2", "entity3", "entity4"}
		actions := []string{"created", "updated", "deleted", "processed"}

		done := make(chan bool, len(entities)*len(actions))
		for _, entity := range entities {
			for _, action := range actions {
				go func(e, a string) {
					for i := 0; i < 100; i++ {
						builder.Build(e, a)
					}
					done <- true
				}(entity, action)
			}
		}

		// 等待所有goroutine完成
		for i := 0; i < len(entities)*len(actions); i++ {
			<-done
		}
	})
}
