package corruption

import "strings"

// ClassifyErr 将错误文本分类为 err_class（用于 corruption 事件与修复路由）。
//
// 说明：
// - 这是跨模块共享逻辑，不属于任何 core 组件的“内部子组件”，因此放在 pkg/utils。
// - 先采用字符串匹配做最小可用闭环，后续可演进为哨兵错误/错误码体系。
func ClassifyErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lm := strings.ToLower(msg)

	switch {
	case strings.Contains(lm, "utxo") || strings.Contains(msg, "状态根") || strings.Contains(msg, "state root"):
		return "utxo_inconsistent"
	case strings.Contains(msg, "获取交易位置失败") || strings.Contains(msg, "交易位置数据格式错误"):
		return "tx_index_corrupt"
	case strings.Contains(msg, "区块高度数据格式错误") || (strings.Contains(lm, "len") && strings.Contains(lm, "8")):
		return "index_corrupt_hash_height"
	case strings.Contains(msg, "区块索引数据格式错误"):
		return "index_corrupt_height_index"
	// 🆕 优先识别"路径损坏"类型（索引中存储了非法路径，如 ../blocks/...）
	// 这种情况需要重建索引，而不是真正的文件缺失
	case strings.Contains(msg, "非法路径") || strings.Contains(msg, "禁止越界访问"):
		return "index_path_corrupt"
	case strings.Contains(msg, "读取区块文件失败") || strings.Contains(lm, "file not found"):
		return "block_file_missing"
	case strings.Contains(msg, "区块文件大小不匹配"):
		return "block_file_size_mismatch"
	case strings.Contains(msg, "反序列化区块失败") || strings.Contains(lm, "unmarshal"):
		return "block_bytes_corrupt"
	case strings.Contains(msg, "链尖数据格式错误") || strings.Contains(msg, "获取链尖状态失败"):
		return "tip_inconsistent"
	default:
		return "unknown"
	}
}


