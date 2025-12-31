package repair

// Options persistence 内部自愈配置（来自 blockchain.sync.advanced 的 repair_* 字段）
type Options struct {
	Enabled         bool
	MaxConcurrency  int
	ThrottleSeconds int
	HashIndexWindow int
}


