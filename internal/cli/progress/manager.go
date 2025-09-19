package progress

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Manager struct {
	mu              sync.Mutex
	enabled         bool
	started         bool
	finished        bool
	stepsOrder      []string
	stepIndexByName map[string]int
	completedIndex  int
	lastRenderText  string
	startTime       time.Time
}

var defaultMgr *Manager

func DefaultSteps() []string {
	return []string{
		"加载配置",
		"创建临时配置",
		"装配模块",
		"启动基础设施",
		"启动通信与数据",
		"启动业务逻辑",
		"启动API",
		"启动CLI",
		"依赖注入检查",
		"初始化界面",
	}
}

func Init(steps []string, enabled bool) {
	if len(steps) == 0 {
		steps = DefaultSteps()
	}
	idx := make(map[string]int, len(steps))
	for i, name := range steps {
		idx[name] = i
	}
	defaultMgr = &Manager{
		enabled:         enabled,
		stepsOrder:      steps,
		stepIndexByName: idx,
		completedIndex:  -1,
	}
}

func Start() {
	if defaultMgr == nil {
		Init(DefaultSteps(), true)
	}
	defaultMgr.mu.Lock()
	defer defaultMgr.mu.Unlock()

	if defaultMgr.started || defaultMgr.finished {
		return
	}
	defaultMgr.started = true
	defaultMgr.startTime = time.Now()

	if !defaultMgr.enabled {
		fmt.Println("⏳ 正在启动，请稍候…")
		return
	}

	fmt.Println(defaultMgr.renderText())
}

func MarkStep(stepName string) {
	if defaultMgr == nil {
		return
	}
	defaultMgr.mu.Lock()
	defer defaultMgr.mu.Unlock()

	if defaultMgr.finished {
		return
	}

	index, ok := defaultMgr.stepIndexByName[stepName]
	if !ok {
		return
	}
	if index <= defaultMgr.completedIndex {
		return
	}
	defaultMgr.completedIndex = index
	fmt.Println(defaultMgr.renderText())
}

func FinishSuccess(message string) {
	if defaultMgr == nil {
		return
	}
	defaultMgr.mu.Lock()
	defer defaultMgr.mu.Unlock()
	if defaultMgr.finished {
		return
	}
	defaultMgr.finished = true
	fmt.Println(defaultMgr.withElapsed(message))
	// 统一由页面工具控制清屏
}

func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func (m *Manager) renderText() string {
	total := len(m.stepsOrder)
	next := m.nextStepName()
	current := m.currentStepName()
	if m.completedIndex < 0 {
		return fmt.Sprintf("正在启动（0/%d）：%s…", total, next)
	}
	if m.completedIndex >= total-1 {
		return fmt.Sprintf("正在完成（%d/%d）：%s…", total, total, current)
	}
	return fmt.Sprintf("正在启动（%d/%d）：%s…", m.completedIndex+1, total, next)
}

func (m *Manager) currentStepName() string {
	if m.completedIndex < 0 {
		return ""
	}
	if m.completedIndex >= len(m.stepsOrder) {
		return m.stepsOrder[len(m.stepsOrder)-1]
	}
	return m.stepsOrder[m.completedIndex]
}

func (m *Manager) nextStepName() string {
	nextIdx := m.completedIndex + 1
	if nextIdx >= len(m.stepsOrder) {
		return m.stepsOrder[len(m.stepsOrder)-1]
	}
	return m.stepsOrder[nextIdx]
}

func (m *Manager) withElapsed(message string) string {
	if m.startTime.IsZero() {
		return message
	}
	elapsed := time.Since(m.startTime).Round(100 * time.Millisecond)
	return fmt.Sprintf("%s（耗时 %s）", message, elapsed)
}
