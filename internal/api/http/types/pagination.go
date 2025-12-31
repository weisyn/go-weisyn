// Package types provides HTTP pagination type definitions.
package types

// PaginationRequest 分页请求参数
type PaginationRequest struct {
	Page     int `form:"page" binding:"omitempty,min=1"`             // 页码（从1开始）
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100"` // 每页数量（最大100）
}

// DefaultPagination 返回默认分页参数
func DefaultPagination() *PaginationRequest {
	return &PaginationRequest{
		Page:     1,
		PageSize: 20,
	}
}

// Offset 计算偏移量
func (p *PaginationRequest) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Limit 返回限制数量
func (p *PaginationRequest) Limit() int {
	return p.PageSize
}

// PaginationResponse 分页响应
type PaginationResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// PaginationMeta 分页元数据
type PaginationMeta struct {
	Page       int   `json:"page"`       // 当前页码
	PageSize   int   `json:"pageSize"`   // 每页数量
	TotalItems int64 `json:"totalItems"` // 总条目数
	TotalPages int   `json:"totalPages"` // 总页数
	HasNext    bool  `json:"hasNext"`    // 是否有下一页
	HasPrev    bool  `json:"hasPrev"`    // 是否有上一页
}

// NewPaginationResponse 创建分页响应
func NewPaginationResponse(data interface{}, page, pageSize int, totalItems int64) *PaginationResponse {
	totalPages := int((totalItems + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	return &PaginationResponse{
		Data: data,
		Pagination: PaginationMeta{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	}
}
