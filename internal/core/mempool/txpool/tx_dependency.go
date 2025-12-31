// 文件说明：
// 本文件定义交易依赖管理器，用于在存储层追踪交易间的依赖/被依赖关系，
// 支撑拓扑排序、按依赖顺序选择、循环依赖检测等能力。
// 职责限定：仅维护依赖图数据结构与查询，不参与业务验证与执行调度。
package txpool

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// DependencyManager 管理交易间的依赖关系
// 维护两张映射：
// - dependents: 某交易被哪些交易依赖；
// - dependencies: 某交易依赖哪些交易。
type DependencyManager struct {
	dependents   map[string]map[string]struct{}
	dependencies map[string]map[string]struct{}
}

// NewDependencyManager 创建新的依赖管理器。
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{dependents: make(map[string]map[string]struct{}), dependencies: make(map[string]map[string]struct{})}
}

// AddTransaction 添加交易并分析其依赖。
// 参数：
// - tx：交易对象；
// - txID：交易ID。
func (dm *DependencyManager) AddTransaction(tx *transaction.Transaction, txID []byte) {
	txIDStr := string(txID)
	if _, exists := dm.dependencies[txIDStr]; !exists {
		dm.dependencies[txIDStr] = make(map[string]struct{})
	}
	for _, input := range tx.Inputs {
		if input.IsReferenceOnly {
			continue
		}
		depTxIDStr := string(input.PreviousOutput.TxId)
		dm.dependencies[txIDStr][depTxIDStr] = struct{}{}
		if _, exists := dm.dependents[depTxIDStr]; !exists {
			dm.dependents[depTxIDStr] = make(map[string]struct{})
		}
		dm.dependents[depTxIDStr][txIDStr] = struct{}{}
	}
}

// RemoveTransaction 移除交易及其依赖关系。
func (dm *DependencyManager) RemoveTransaction(txID []byte) {
	txIDStr := string(txID)
	if deps, exists := dm.dependencies[txIDStr]; exists {
		for depID := range deps {
			if depts, ok := dm.dependents[depID]; ok {
				delete(depts, txIDStr)
				if len(depts) == 0 {
					delete(dm.dependents, depID)
				}
			}
		}
		delete(dm.dependencies, txIDStr)
	}
	if depts, exists := dm.dependents[txIDStr]; exists {
		for deptID := range depts {
			if deps, ok := dm.dependencies[deptID]; ok {
				delete(deps, txIDStr)
				if len(deps) == 0 {
					delete(dm.dependencies, deptID)
				}
			}
		}
		delete(dm.dependents, txIDStr)
	}
}

// GetDependencies 获取交易的所有依赖交易ID。
// 返回：依赖的交易ID切片。
func (dm *DependencyManager) GetDependencies(txID []byte) [][]byte {
	txIDStr := string(txID)
	result := make([][]byte, 0)
	if deps, exists := dm.dependencies[txIDStr]; exists {
		for depID := range deps {
			result = append(result, []byte(depID))
		}
	}
	return result
}

// GetDependents 获取依赖此交易的所有交易ID。
// 返回：被依赖者列表。
func (dm *DependencyManager) GetDependents(txID []byte) [][]byte {
	txIDStr := string(txID)
	result := make([][]byte, 0)
	if depts, exists := dm.dependents[txIDStr]; exists {
		for deptID := range depts {
			result = append(result, []byte(deptID))
		}
	}
	return result
}

// HasDependencies 检查交易是否有依赖。
func (dm *DependencyManager) HasDependencies(txID []byte) bool {
	txIDStr := string(txID)
	if deps, exists := dm.dependencies[txIDStr]; exists {
		return len(deps) > 0
	}
	return false
}

// HasDependents 检查是否有交易依赖此交易。
func (dm *DependencyManager) HasDependents(txID []byte) bool {
	txIDStr := string(txID)
	if depts, exists := dm.dependents[txIDStr]; exists {
		return len(depts) > 0
	}
	return false
}

// BuildDependencyGraph 构建某交易的完整依赖子图。
// 返回：节点->依赖列表的映射。
func (dm *DependencyManager) BuildDependencyGraph(rootTxID []byte) map[string][]string {
	graph := make(map[string][]string)
	visited := make(map[string]struct{})
	dm.buildGraphRecursive(string(rootTxID), graph, visited)
	return graph
}

// 递归构建依赖图（内部）。
func (dm *DependencyManager) buildGraphRecursive(txID string, graph map[string][]string, visited map[string]struct{}) {
	if _, alreadyVisited := visited[txID]; alreadyVisited {
		return
	}
	visited[txID] = struct{}{}
	if deps, exists := dm.dependencies[txID]; exists && len(deps) > 0 {
		depList := make([]string, 0, len(deps))
		for depID := range deps {
			depList = append(depList, depID)
			if _, depExists := dm.dependencies[depID]; depExists {
				dm.buildGraphRecursive(depID, graph, visited)
			}
		}
		graph[txID] = depList
	} else {
		graph[txID] = []string{}
	}
}

// GetTransactionsByDependencyOrder 按依赖顺序排序交易ID列表（拓扑排序）。
func (dm *DependencyManager) GetTransactionsByDependencyOrder(txIDs [][]byte) [][]byte {
	graph := make(map[string][]string)
	for _, txID := range txIDs {
		subGraph := dm.BuildDependencyGraph(txID)
		for nodeID, deps := range subGraph {
			if _, exists := graph[nodeID]; !exists {
				graph[nodeID] = deps
			} else {
				existing := make(map[string]struct{})
				for _, d := range graph[nodeID] {
					existing[d] = struct{}{}
				}
				for _, d := range deps {
					if _, ok := existing[d]; !ok {
						graph[nodeID] = append(graph[nodeID], d)
						existing[d] = struct{}{}
					}
				}
			}
		}
	}
	return dm.topologicalSort(graph)
}

// 拓扑排序（Kahn算法）。
func (dm *DependencyManager) topologicalSort(graph map[string][]string) [][]byte {
	inDegree := make(map[string]int)
	for node := range graph {
		inDegree[node] = 0
	}
	for node, deps := range graph {
		for _, dep := range deps {
			if _, exists := graph[dep]; exists {
				inDegree[node]++
			}
		}
	}
	queue := make([]string, 0)
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}
	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		for child, deps := range graph {
			for _, dep := range deps {
				if dep == node {
					inDegree[child]--
					if inDegree[child] == 0 {
						queue = append(queue, child)
					}
					break
				}
			}
		}
	}
	byteResult := make([][]byte, len(result))
	for i, id := range result {
		byteResult[i] = []byte(id)
	}
	return byteResult
}

// DetectCycles 检测依赖图中是否存在循环依赖。
func (dm *DependencyManager) DetectCycles() bool {
	visited := make(map[string]struct{})
	recStack := make(map[string]struct{})
	for node := range dm.dependencies {
		if _, ok := visited[node]; !ok {
			if dm.isCyclicUtil(node, visited, recStack) {
				return true
			}
		}
	}
	return false
}

// DFS辅助：检测环。
func (dm *DependencyManager) isCyclicUtil(node string, visited, recStack map[string]struct{}) bool {
	visited[node] = struct{}{}
	recStack[node] = struct{}{}
	if deps, exists := dm.dependencies[node]; exists {
		for dep := range deps {
			if _, seen := visited[dep]; !seen {
				if dm.isCyclicUtil(dep, visited, recStack) {
					return true
				}
			} else if _, inStack := recStack[dep]; inStack {
				return true
			}
		}
	}
	delete(recStack, node)
	return false
}

// ResolveDependencies 解析交易依赖，返回可执行序列（过滤已存在的依赖）。
func (dm *DependencyManager) ResolveDependencies(txIDs [][]byte, existingTxs map[string]struct{}) [][]byte {
	subGraph := make(map[string][]string)
	for _, txID := range txIDs {
		txIDStr := string(txID)
		if deps, exists := dm.dependencies[txIDStr]; exists {
			depList := make([]string, 0)
			for dep := range deps {
				if _, hasExisting := existingTxs[dep]; !hasExisting {
					depList = append(depList, dep)
				}
			}
			subGraph[txIDStr] = depList
		} else {
			subGraph[txIDStr] = []string{}
		}
	}
	return dm.topologicalSort(subGraph)
}
