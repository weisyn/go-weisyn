package constants

// PeerstoreKeyChainIdentity caches a peer's ChainIdentity in the local peerstore.
//
// 设计目标：
// - 让“链身份判定”优先走系统路径（SyncHelloV2 / KBucketSync 响应中携带的 ChainIdentity），而不是依赖易缺失/易变化的 UserAgent；
// - 该值仅作为本地缓存与准入判定依据，不作为网络协议字段。
//
// 约定：
// - value 类型为 string（JSON 编码的 pkg/types.ChainIdentity）。
const PeerstoreKeyChainIdentity = "weisyn.chain_identity"
