package toolbelt

import (
	"fmt"
	"github.com/y-a-r-g/json2flag"
	"os"
	"os/signal"
	"reflect"
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
	tools          map[reflect.Type]ITool
	toolsLock      sync.Locker
	toolOrder      []reflect.Type
	interrupt      chan os.Signal
	running        bool
	configFileName string
}

func NewBelt(configFileName string) IBelt {
	return &belt{
		tools:          map[reflect.Type]ITool{},
		toolsLock:      &sync.Mutex{},
		interrupt:      make(chan os.Signal),
		configFileName: configFileName,
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
	b.toolsLock.Unlock()

	err := json2flag.ReadConfigFile(b.configFileName)
	if err != nil {
		defaultName := b.configFileName + ".default"
		err := json2flag.WriteConfigFile(defaultName, 0644)
		if err != nil {
			panic(err)
		}
		panic(fmt.Sprintf("cannot read config from: %s. Default config is written to: %s", b.configFileName, defaultName))
	}

	for _, tt := range b.toolOrder {
		b.Tool(tt).Start(b)
	}

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

	var sorted []reflect.Type
	var visited []reflect.Type

	var visit func(item reflect.Type)
	visit = func(item reflect.Type) {
		visitedContainsItem := false
		for _, i := range visited {
			if i == item {
				visitedContainsItem = true
				break
			}
		}

		if !visitedContainsItem {
			visited = append(visited, item)
			for _, dep := range deps[item] {
				visit(dep)
			}

			sorted = append(sorted, item)
		} else {
			for _, i := range sorted {
				if i == item {
					return
				}
			}
			panic(fmt.Sprintf("cyclic dependencies in tool: %s", item))
		}
	}

	for tt := range b.tools {
		visit(tt)
	}

	b.toolOrder = sorted
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
