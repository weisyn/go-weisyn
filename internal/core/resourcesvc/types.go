package resourcesvc

// NOTE: 这些类型的对外定义已迁移到 `pkg/interfaces/resourcesvc`。
// 这里保留类型别名以保持内部实现的兼容性，避免重复定义。

import resourcesvciface "github.com/weisyn/v1/pkg/interfaces/resourcesvc"

type ResourceView = resourcesvciface.ResourceView
type ResourceViewFilter = resourcesvciface.ResourceViewFilter
type PageRequest = resourcesvciface.PageRequest
type PageResponse = resourcesvciface.PageResponse
type TxSummary = resourcesvciface.TxSummary
type ReferenceSummary = resourcesvciface.ReferenceSummary
type ResourceHistory = resourcesvciface.ResourceHistory
