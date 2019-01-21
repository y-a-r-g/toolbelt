package toolbelt

import (
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/y-a-r-g/json2flag"
	"reflect"
	"runtime"
)

type RedisTool struct {
	*pool.Pool
	config *RedisConfig
}

type RedisConfig struct {
	Url, Auth string
	Debug     bool
	DbIndex   uint
	PoolSize  uint
}

var TRedis = reflect.TypeOf(RedisTool{})

func (t *RedisTool) Configure(config ...interface{}) {
	t.config = &RedisConfig{
		Debug:    false,
		Url:      "localhost:6379",
		Auth:     "",
		DbIndex:  0,
		PoolSize: uint(runtime.GOMAXPROCS(0)),
	}

	if len(config) > 0 {
		*t.config = *config[0].(*RedisConfig)
	}

	json2flag.FlagPrefixed(t.config, map[string]string{
		"Debug": "debug mode",
	}, TRedis.String())
}

func (t *RedisTool) Dependencies() []reflect.Type {
	return nil
}

func (t *RedisTool) Start(belt IBelt) {
	dial := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		if t.config.Auth != "" {
			if err = client.Cmd("AUTH", t.config.Auth).Err; err != nil {
				_ = client.Close()
				return nil, err
			}
		}

		if err = client.Cmd("SELECT", t.config.DbIndex).Err; err != nil {
			_ = client.Close()
			return nil, err
		}

		return client, nil
	}

	redisPool, err := pool.NewCustom("tcp", t.config.Url, int(t.config.PoolSize), dial)
	if err != nil {
		panic(err)
	}
	t.Pool = redisPool
}

func (t *RedisTool) Stop(belt IBelt) {
	t.Pool.Empty()
	t.Pool = nil
}
