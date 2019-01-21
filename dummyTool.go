package toolbelt

import (
	"github.com/y-a-r-g/json2flag"
	"reflect"
)

type DummyTool struct {
	config *DummyConfig
}

type DummyConfig struct {
	Debug bool
}

var TDummy = reflect.TypeOf(DummyTool{})

func (t *DummyTool) Configure(config ...interface{}) {
	t.config = &DummyConfig{}

	if len(config) > 0 {
		*t.config = *config[0].(*DummyConfig)
	}

	json2flag.FlagPrefixed(t.config, map[string]string{
		"Debug": "debug mode",
	}, TDummy.String())
}

func (t *DummyTool) Dependencies() []reflect.Type {
	return nil
}

func (t *DummyTool) Start(belt IBelt) {
}

func (t *DummyTool) Stop(belt IBelt) {
}
