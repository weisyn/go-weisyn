package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

/*
🎯 查询引擎模块

这个模块展示如何在应用中构建高效的数据查询系统：
1. 建立多维度数据索引
2. 实现快速搜索和筛选
3. 支持复杂查询条件组合
4. 优化查询性能

💡 实际应用考虑：
- 支持分布式索引
- 实现缓存机制
- 提供查询优化器
- 支持实时索引更新
*/

// QueryEngine 查询引擎
type QueryEngine struct {
	titleIndex    map[string][]string            // 标题索引：title -> []recordID
	tagIndex      map[string][]string            // 标签索引：tag -> []recordID
	ownerIndex    map[string][]string            // 所有者索引：owner -> []recordID
	typeIndex     map[string][]string            // 类型索引：dataType -> []recordID
	timeIndex     map[string][]string            // 时间索引：date -> []recordID
	metadataIndex map[string]map[string][]string // 元数据索引：key -> value -> []recordID
	recordCache   map[string]*DataRecord         // 记录缓存：recordID -> record
}

// IndexStats 索引统计信息
type IndexStats struct {
	TotalRecords    int `json:"total_records"`
	TitleEntries    int `json:"title_entries"`
	TagEntries      int `json:"tag_entries"`
	OwnerEntries    int `json:"owner_entries"`
	TypeEntries     int `json:"type_entries"`
	MetadataEntries int `json:"metadata_entries"`
	CacheSize       int `json:"cache_size"`
}

// SearchResult 搜索结果
type SearchResult struct {
	RecordIDs   []string `json:"record_ids"`  // 匹配的记录ID列表
	TotalCount  int      `json:"total_count"` // 总匹配数量
	SearchTime  int64    `json:"search_time"` // 搜索耗时(微秒)
	IndexUsed   []string `json:"index_used"`  // 使用的索引
	Explanation string   `json:"explanation"` // 查询解释
}

// NewQueryEngine 创建新的查询引擎
func NewQueryEngine() *QueryEngine {
	return &QueryEngine{
		titleIndex:    make(map[string][]string),
		tagIndex:      make(map[string][]string),
		ownerIndex:    make(map[string][]string),
		typeIndex:     make(map[string][]string),
		timeIndex:     make(map[string][]string),
		metadataIndex: make(map[string]map[string][]string),
		recordCache:   make(map[string]*DataRecord),
	}
}

// AddToIndex 将记录添加到索引
// 🎯 功能：为新记录建立多维度索引
func (qe *QueryEngine) AddToIndex(record DataRecord) error {
	recordID := record.ID

	// 💡 生活化理解：
	// 建立索引就像整理图书馆
	// - 按标题分类 = 标题索引
	// - 按作者分类 = 所有者索引
	// - 按类型分类 = 数据类型索引
	// - 按时间分类 = 时间索引

	// 📋 步骤1：标题索引
	if record.Title != "" {
		titleKey := strings.ToLower(record.Title)
		qe.titleIndex[titleKey] = qe.addToStringSlice(qe.titleIndex[titleKey], recordID)

		// 支持部分匹配（按单词分割）
		words := strings.Fields(titleKey)
		for _, word := range words {
			if len(word) > 2 { // 忽略太短的词
				qe.titleIndex[word] = qe.addToStringSlice(qe.titleIndex[word], recordID)
			}
		}
	}

	// 📋 步骤2：标签索引
	for _, tag := range record.Tags {
		if tag != "" {
			tagKey := strings.ToLower(tag)
			qe.tagIndex[tagKey] = qe.addToStringSlice(qe.tagIndex[tagKey], recordID)
		}
	}

	// 📋 步骤3：所有者索引
	if record.Owner != "" {
		qe.ownerIndex[record.Owner] = qe.addToStringSlice(qe.ownerIndex[record.Owner], recordID)
	}

	// 📋 步骤4：数据类型索引
	if record.DataType != "" {
		typeKey := strings.ToLower(record.DataType)
		qe.typeIndex[typeKey] = qe.addToStringSlice(qe.typeIndex[typeKey], recordID)
	}

	// 📋 步骤5：时间索引（按日期）
	dateKey := record.Timestamp.Format("2006-01-02")
	qe.timeIndex[dateKey] = qe.addToStringSlice(qe.timeIndex[dateKey], recordID)

	// 📋 步骤6：元数据索引
	for key, value := range record.Metadata {
		if qe.metadataIndex[key] == nil {
			qe.metadataIndex[key] = make(map[string][]string)
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr != "" {
			qe.metadataIndex[key][valueStr] = qe.addToStringSlice(qe.metadataIndex[key][valueStr], recordID)
		}
	}

	// 📋 步骤7：缓存记录
	qe.recordCache[recordID] = &record

	return nil
}

// SearchIndex 在索引中搜索
// 🎯 功能：基于索引进行快速搜索
func (qe *QueryEngine) SearchIndex(request QueryRequest) ([]string, error) {
	startTime := time.Now()
	var resultSets [][]string
	var indexUsed []string

	// 📋 步骤1：按ID精确查询（最高优先级）
	if request.ID != "" {
		if _, exists := qe.recordCache[request.ID]; exists {
			result := &SearchResult{
				RecordIDs:   []string{request.ID},
				TotalCount:  1,
				SearchTime:  time.Since(startTime).Microseconds(),
				IndexUsed:   []string{"id_cache"},
				Explanation: "ID精确匹配",
			}
			return result.RecordIDs, nil
		}
		return []string{}, nil // ID不存在
	}

	// 📋 步骤2：标题搜索
	if request.Title != "" {
		titleResults := qe.searchInIndex(qe.titleIndex, strings.ToLower(request.Title))
		if len(titleResults) > 0 {
			resultSets = append(resultSets, titleResults)
			indexUsed = append(indexUsed, "title")
		}
	}

	// 📋 步骤3：标签搜索
	if len(request.Tags) > 0 {
		var tagResults []string
		for _, tag := range request.Tags {
			if tag != "" {
				tagKey := strings.ToLower(tag)
				results := qe.searchInIndex(qe.tagIndex, tagKey)
				tagResults = qe.mergeStringSlices(tagResults, results)
			}
		}
		if len(tagResults) > 0 {
			resultSets = append(resultSets, tagResults)
			indexUsed = append(indexUsed, "tags")
		}
	}

	// 📋 步骤4：所有者搜索
	if request.Owner != "" {
		ownerResults := qe.searchInIndex(qe.ownerIndex, request.Owner)
		if len(ownerResults) > 0 {
			resultSets = append(resultSets, ownerResults)
			indexUsed = append(indexUsed, "owner")
		}
	}

	// 📋 步骤5：数据类型搜索
	if request.DataType != "" {
		typeKey := strings.ToLower(request.DataType)
		typeResults := qe.searchInIndex(qe.typeIndex, typeKey)
		if len(typeResults) > 0 {
			resultSets = append(resultSets, typeResults)
			indexUsed = append(indexUsed, "data_type")
		}
	}

	// 📋 步骤6：时间范围搜索
	if !request.TimeFrom.IsZero() || !request.TimeTo.IsZero() {
		timeResults := qe.searchTimeRange(request.TimeFrom, request.TimeTo)
		if len(timeResults) > 0 {
			resultSets = append(resultSets, timeResults)
			indexUsed = append(indexUsed, "time_range")
		}
	}

	// 📋 步骤7：元数据搜索
	for key, value := range request.Metadata {
		if key != "" && value != "" {
			if metaIndex, exists := qe.metadataIndex[key]; exists {
				metaResults := qe.searchInIndex(metaIndex, value)
				if len(metaResults) > 0 {
					resultSets = append(resultSets, metaResults)
					indexUsed = append(indexUsed, fmt.Sprintf("metadata.%s", key))
				}
			}
		}
	}

	// 📋 步骤8：求交集（AND逻辑）
	var finalResults []string
	if len(resultSets) == 0 {
		// 没有搜索条件，返回所有记录
		for recordID := range qe.recordCache {
			finalResults = append(finalResults, recordID)
		}
	} else {
		finalResults = qe.intersectStringSlices(resultSets)
	}

	// 📋 步骤9：应用限制
	if request.Limit > 0 && len(finalResults) > request.Limit {
		finalResults = finalResults[:request.Limit]
	}

	return finalResults, nil
}

// UpdateIndex 更新索引中的记录
func (qe *QueryEngine) UpdateIndex(record DataRecord) error {
	// 先移除旧索引
	if err := qe.RemoveFromIndex(record.ID); err != nil {
		return fmt.Errorf("移除旧索引失败: %v", err)
	}

	// 再添加新索引
	return qe.AddToIndex(record)
}

// RemoveFromIndex 从索引中移除记录
func (qe *QueryEngine) RemoveFromIndex(recordID string) error {
	// 从缓存中获取记录信息
	record, exists := qe.recordCache[recordID]
	if !exists {
		return nil // 记录不存在，无需移除
	}

	// 从各个索引中移除
	qe.removeFromStringIndex(qe.titleIndex, strings.ToLower(record.Title), recordID)

	for _, tag := range record.Tags {
		qe.removeFromStringIndex(qe.tagIndex, strings.ToLower(tag), recordID)
	}

	qe.removeFromStringIndex(qe.ownerIndex, record.Owner, recordID)
	qe.removeFromStringIndex(qe.typeIndex, strings.ToLower(record.DataType), recordID)

	dateKey := record.Timestamp.Format("2006-01-02")
	qe.removeFromStringIndex(qe.timeIndex, dateKey, recordID)

	// 从元数据索引中移除
	for key, value := range record.Metadata {
		if metaIndex, exists := qe.metadataIndex[key]; exists {
			valueStr := fmt.Sprintf("%v", value)
			qe.removeFromStringIndex(metaIndex, valueStr, recordID)
		}
	}

	// 从缓存中删除
	delete(qe.recordCache, recordID)

	return nil
}

// GetIndexStats 获取索引统计信息
func (qe *QueryEngine) GetIndexStats() IndexStats {
	return IndexStats{
		TotalRecords:    len(qe.recordCache),
		TitleEntries:    len(qe.titleIndex),
		TagEntries:      len(qe.tagIndex),
		OwnerEntries:    len(qe.ownerIndex),
		TypeEntries:     len(qe.typeIndex),
		MetadataEntries: len(qe.metadataIndex),
		CacheSize:       len(qe.recordCache),
	}
}

// OptimizeIndex 优化索引结构
func (qe *QueryEngine) OptimizeIndex() error {
	// 清理空的索引项
	qe.cleanEmptyEntries(qe.titleIndex)
	qe.cleanEmptyEntries(qe.tagIndex)
	qe.cleanEmptyEntries(qe.ownerIndex)
	qe.cleanEmptyEntries(qe.typeIndex)
	qe.cleanEmptyEntries(qe.timeIndex)

	// 清理元数据索引
	for key, metaIndex := range qe.metadataIndex {
		qe.cleanEmptyEntries(metaIndex)
		if len(metaIndex) == 0 {
			delete(qe.metadataIndex, key)
		}
	}

	return nil
}

// 私有方法：在字符串索引中搜索
func (qe *QueryEngine) searchInIndex(index map[string][]string, key string) []string {
	if results, exists := index[key]; exists {
		return qe.copyStringSlice(results)
	}
	return []string{}
}

// 私有方法：时间范围搜索
func (qe *QueryEngine) searchTimeRange(from, to time.Time) []string {
	var results []string

	for dateStr, recordIDs := range qe.timeIndex {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// 检查是否在时间范围内
		if (from.IsZero() || date.After(from) || date.Equal(from)) &&
			(to.IsZero() || date.Before(to) || date.Equal(to)) {
			results = qe.mergeStringSlices(results, recordIDs)
		}
	}

	return results
}

// 私有方法：添加到字符串切片（避免重复）
func (qe *QueryEngine) addToStringSlice(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice // 已存在
		}
	}
	return append(slice, item)
}

// 私有方法：合并字符串切片
func (qe *QueryEngine) mergeStringSlices(slice1, slice2 []string) []string {
	result := qe.copyStringSlice(slice1)
	for _, item := range slice2 {
		result = qe.addToStringSlice(result, item)
	}
	return result
}

// 私有方法：求字符串切片的交集
func (qe *QueryEngine) intersectStringSlices(slices [][]string) []string {
	if len(slices) == 0 {
		return []string{}
	}

	if len(slices) == 1 {
		return qe.copyStringSlice(slices[0])
	}

	// 从最小的集合开始
	result := qe.copyStringSlice(slices[0])

	for i := 1; i < len(slices); i++ {
		result = qe.intersectTwoSlices(result, slices[i])
		if len(result) == 0 {
			break // 没有交集
		}
	}

	return result
}

// 私有方法：求两个字符串切片的交集
func (qe *QueryEngine) intersectTwoSlices(slice1, slice2 []string) []string {
	var result []string

	for _, item1 := range slice1 {
		for _, item2 := range slice2 {
			if item1 == item2 {
				result = append(result, item1)
				break
			}
		}
	}

	return result
}

// 私有方法：复制字符串切片
func (qe *QueryEngine) copyStringSlice(slice []string) []string {
	if slice == nil {
		return []string{}
	}
	result := make([]string, len(slice))
	copy(result, slice)
	return result
}

// 私有方法：从字符串索引中移除项目
func (qe *QueryEngine) removeFromStringIndex(index map[string][]string, key, item string) {
	if slice, exists := index[key]; exists {
		newSlice := []string{}
		for _, existing := range slice {
			if existing != item {
				newSlice = append(newSlice, existing)
			}
		}

		if len(newSlice) == 0 {
			delete(index, key)
		} else {
			index[key] = newSlice
		}
	}
}

// 私有方法：清理空索引项
func (qe *QueryEngine) cleanEmptyEntries(index map[string][]string) {
	for key, slice := range index {
		if len(slice) == 0 {
			delete(index, key)
		}
	}
}

// 演示函数：展示查询引擎功能
func DemoQueryEngine() {
	fmt.Println("🎮 查询引擎演示")
	fmt.Println("===============")

	// 创建查询引擎
	qe := NewQueryEngine()

	// 1. 添加测试数据
	fmt.Println("1. 添加测试数据...")
	testRecords := []DataRecord{
		{
			ID:        "doc1",
			Title:     "Go语言教程",
			Content:   "这是一个Go语言的入门教程",
			DataType:  "document",
			Owner:     "alice",
			Tags:      []string{"编程", "Go", "教程"},
			Metadata:  map[string]interface{}{"难度": "初级", "页数": 50},
			Timestamp: time.Now().AddDate(0, 0, -1),
		},
		{
			ID:        "doc2",
			Title:     "区块链基础",
			Content:   "区块链技术的基础知识介绍",
			DataType:  "document",
			Owner:     "bob",
			Tags:      []string{"区块链", "技术", "基础"},
			Metadata:  map[string]interface{}{"难度": "中级", "页数": 100},
			Timestamp: time.Now(),
		},
		{
			ID:        "img1",
			Title:     "系统架构图",
			Content:   "base64_encoded_image_data",
			DataType:  "image",
			Owner:     "alice",
			Tags:      []string{"架构", "设计"},
			Metadata:  map[string]interface{}{"格式": "PNG", "大小": "2MB"},
			Timestamp: time.Now().AddDate(0, 0, -2),
		},
	}

	for _, record := range testRecords {
		err := qe.AddToIndex(record)
		if err != nil {
			fmt.Printf("添加索引失败: %v\n", err)
			return
		}
	}

	fmt.Printf("添加了 %d 条记录\n", len(testRecords))

	// 2. 标题搜索演示
	fmt.Println("\n2. 标题搜索演示...")
	titleQuery := QueryRequest{Title: "Go"}
	results, err := qe.SearchIndex(titleQuery)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}
	fmt.Printf("搜索'Go': 找到 %d 个结果 %v\n", len(results), results)

	// 3. 标签搜索演示
	fmt.Println("\n3. 标签搜索演示...")
	tagQuery := QueryRequest{Tags: []string{"技术"}}
	results, err = qe.SearchIndex(tagQuery)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}
	fmt.Printf("搜索标签'技术': 找到 %d 个结果 %v\n", len(results), results)

	// 4. 所有者搜索演示
	fmt.Println("\n4. 所有者搜索演示...")
	ownerQuery := QueryRequest{Owner: "alice"}
	results, err = qe.SearchIndex(ownerQuery)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}
	fmt.Printf("搜索所有者'alice': 找到 %d 个结果 %v\n", len(results), results)

	// 5. 复合查询演示
	fmt.Println("\n5. 复合查询演示...")
	complexQuery := QueryRequest{
		Owner:    "alice",
		DataType: "document",
		Tags:     []string{"Go"},
	}
	results, err = qe.SearchIndex(complexQuery)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}
	fmt.Printf("复合查询: 找到 %d 个结果 %v\n", len(results), results)

	// 6. 索引统计演示
	fmt.Println("\n6. 索引统计演示...")
	stats := qe.GetIndexStats()
	statsJSON, _ := json.MarshalIndent(stats, "", "  ")
	fmt.Printf("索引统计: %s\n", statsJSON)

	fmt.Println("✅ 查询引擎演示完成")
}
