package adapter

import (
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

// memoryAllocator 简单的 bump allocator
// 从 WASM 线性内存的高地址向下分配，避免与栈冲突
type memoryAllocator struct {
	currentTop uint32 // 当前可分配的顶部位置
	guardSize  uint32 // 保护区大小（避免与栈冲突，默认8KB）
	mutex      sync.Mutex
}

// allocate 从 WASM 内存分配指定大小的空间
func (alloc *memoryAllocator) allocate(memory api.Memory, size uint32) (uint32, error) {
	alloc.mutex.Lock()
	defer alloc.mutex.Unlock()

	// ⚠️ **零大小处理**：零大小分配对齐到最小分配单位（8字节）
	// 这样可以确保返回的指针是有效的，并且对齐到8字节边界
	if size == 0 {
		size = 8
	}

	// 对齐到 8 字节边界（提升性能，避免未对齐访问）
	alignedSize := (size + 7) & ^uint32(7)

	// 检查是否有足够空间
	memSize := uint32(memory.Size())
	requiredSpace := alignedSize + alloc.guardSize

	if alloc.currentTop < requiredSpace {
		// 需要扩容 - 计算需要的页数（每页 64KB）
		additionalBytes := requiredSpace - alloc.currentTop + 65536 // 多分配一页
		pagesNeeded := (additionalBytes + 65535) / 65536

		oldSize, success := memory.Grow(pagesNeeded)
		if !success {
			return 0, fmt.Errorf("内存扩容失败: 需要 %d 页, 当前 %d 页", pagesNeeded, oldSize/65536)
		}

		// 扩容成功，更新分配器状态
		newMemSize := uint32(memory.Size())
		alloc.currentTop = newMemSize
		memSize = newMemSize
	}

	// 从顶部向下分配
	alloc.currentTop -= alignedSize
	ptr := alloc.currentTop

	// 确保指针在有效范围内
	if ptr >= memSize {
		return 0, fmt.Errorf("分配的指针越界: ptr=%d, memSize=%d", ptr, memSize)
	}

	return ptr, nil
}

// getOrCreateAllocator 获取或创建模块的内存分配器
func (a *WASMAdapter) getOrCreateAllocator(moduleName string, memory api.Memory) *memoryAllocator {
	a.allocMutex.Lock()
	defer a.allocMutex.Unlock()

	if alloc, exists := a.allocators[moduleName]; exists {
		return alloc
	}

	// 创建新的分配器 - 从内存顶部向下分配，留出 8KB 保护区
	memSize := uint32(memory.Size())
	alloc := &memoryAllocator{
		currentTop: memSize,
		guardSize:  8192, // 8KB 保护区
	}
	a.allocators[moduleName] = alloc

	if a.logger != nil {
		a.logger.Debugf("创建内存分配器: module=%s, memSize=%d bytes (%.2f KB)",
			moduleName, memSize, float64(memSize)/1024)
	}

	return alloc
}

