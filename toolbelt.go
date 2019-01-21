package toolbelt

import (
	"os"
	"os/signal"
	"reflect"
	"sort"
	"sync"
	"syscall"
)

type IBelt interface {
	Tool(toolType reflect.Type, config ...interface{}) ITool
	Serve()
	Shutdown()
}

type ITool interface {
	Configure(config ...interface{})
	Dependencies() []reflect.Type
	Start(belt IBelt)
	Stop(belt IBelt)
}

type belt struct {
	tools     map[reflect.Type]ITool
	toolsLock sync.Locker
	toolOrder []reflect.Type
	interrupt chan os.Signal
	running   bool
}

func NewBelt() IBelt {
	return &belt{
		tools:     map[reflect.Type]ITool{},
		toolsLock: &sync.Mutex{},
		interrupt: make(chan os.Signal),
	}
}

func (b *belt) Tool(toolType reflect.Type, config ...interface{}) ITool {
	b.toolsLock.Lock()
	tool, startRequired := b.toolUnsafe(toolType)
	b.toolsLock.Unlock()

	if startRequired && b.running {
		tool.Configure(config...)
		tool.Start(b)
	}

	return tool
}

func (b *belt) Serve() {
	b.toolsLock.Lock()
	b.satisfyDependencies()

	b.running = true
	for _, tt := range b.toolOrder {
		b.Tool(tt).Start(b)
	}
	defer b.toolsLock.Unlock()

	signal.Notify(b.interrupt, syscall.SIGTERM)
	signal.Notify(b.interrupt, syscall.SIGINT)

	<-b.interrupt

	for _, tt := range b.toolOrder {
		b.Tool(tt).Stop(b)
	}
	b.running = false
}

func (b *belt) Shutdown() {
	select {
	case b.interrupt <- nil:
		break
	default:
		break
	}
}

func (b *belt) satisfyDependencies() {
	deps := map[reflect.Type][]reflect.Type{}

	for len(deps) < len(b.tools) {
		for tt, tool := range b.tools {
			added := false
			for _, t := range b.toolOrder {
				if t == tt {
					added = true
					break
				}
			}
			if !added {
				b.toolOrder = append(b.toolOrder, tt)
				newDeps := tool.Dependencies()
				deps[tt] = newDeps
				for _, depType := range newDeps {
					b.toolUnsafe(depType)
				}
			}
		}
	}

	sort.Slice(b.toolOrder, func(i, j int) bool {
		t := b.toolOrder[i]
		d := deps[b.toolOrder[j]]

		for _, tt := range d {
			if t == tt {
				return true
			}
		}
		return false
	})
}

func (b *belt) toolUnsafe(toolType reflect.Type) (tool ITool, startRequired bool) {
	tool = b.tools[toolType]
	if tool == nil {
		tool = reflect.New(toolType).Interface().(ITool)
		tool.Configure()
		b.tools[toolType] = tool
		startRequired = true
	}
	return
}
