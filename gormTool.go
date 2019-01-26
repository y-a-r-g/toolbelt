package toolbelt

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/y-a-r-g/json2flag"
	"reflect"
)

type GormTool struct {
	*gorm.DB
	config *GormConfig
}

type GormConfig struct {
	Debug          bool
	Dialect        string
	Url            string
	DbMaxIdleConns uint
	DbMaxOpenConns uint
}

var TGorm = reflect.TypeOf(GormTool{})

func (t *GormTool) Configure(config ...interface{}) {
	t.config = &GormConfig{
		Dialect:        "postgres",
		Url:            "host=localhost port=5432 dbname=test sslmode=disable",
		DbMaxOpenConns: 64,
		DbMaxIdleConns: 8,
	}

	if len(config) > 0 {
		*t.config = *config[0].(*GormConfig)
	}

	json2flag.FlagPrefixed(t.config, map[string]string{
		"Debug":          "debug mode",
		"Dialect":        "dialect",
		"Url":            "database url",
		"DbMaxIdleConns": "maximum idle connections",
		"DbMaxOpenConns": "maximum open connections",
	}, TGorm.Name())
}

func (t *GormTool) Dependencies() []reflect.Type {
	return nil
}

func (t *GormTool) Start(belt IBelt) {
	db, err := gorm.Open(t.config.Dialect, t.config.Url)
	if err != nil {
		panic(err)
	}
	db.LogMode(t.config.Debug)
	db.DB().SetMaxIdleConns(int(t.config.DbMaxIdleConns))
	db.DB().SetMaxOpenConns(int(t.config.DbMaxOpenConns))
	t.DB = db
}

func (t *GormTool) Stop(belt IBelt) {
	err := t.Close()
	if err != nil {
		belt.Tool(TLogrus).(*LogrusTool).WithError(err).Error("cannot close db connection")
	}
	t.DB = nil
}
