package broker

import (
	"github.com/go-redis/redis"
	"github.com/shopspring/decimal"
	"github.com/vadiminshakov/exmo"
	"time"
)

type Order struct {
	Closed    bool    `json:"closed"`
	SellPrice float64 `json:"sellprice"`
	BuyPrice  float64 `json:"buyprice"`
	Volume    float64 `json:"volume"`
}

type Meta struct {
	SELLEDNOW  float64 // variable for storing last selled item buy-price
	TRADELIMIT decimal.Decimal
}

type Fabricant struct {
	Ch     chan map[float64]float64
	Conf   Config
	Orders map[float64]Order
	Api    *exmo.Exmo
	Timers
	Meta
	Db *redis.Client
}

type Timers struct {
	POLLINTERVAL time.Duration
	WAITFORBUY   time.Duration
	ORDERSCHECK  time.Duration
}

type Config struct {
	MinPrice float64 `yaml:"minprice"`
	MaxPrice float64 `yaml:"maxprice"`
	Gap      float64 `yaml:"gap"`
	UseRedis bool    `yaml:"useredis"`
	DbAddr   string  `yaml:"dbaddr"`
	DbPort   string  `yaml:dbport`
	DbPass   string  `yaml:"dbpass"`
	DbNum    int     `yaml:"dbnum"`
}
