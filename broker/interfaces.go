package broker

import "github.com/vadiminshakov/exmo"

type Fabricator interface {
	Sell(sell, buy string, volume, price float64) string
	MarketBuy(buy, sell string, volume float64) string
	Buy(buy, sell string, volume, price float64) string
	WaitOrdersExecute()
	WhatICanBuy(buy, sell string) (float64, error)
	WhatICanSell(currency string) float64
	Monitor()
	WaitForBuy(buy, sell string, price, amount float64) string
	Save(key float64, value Order) error
	Delete(key float64)
	GetConfig() Config
	GetTimers() Timers
	GetApi() *exmo.Exmo
	GetMeta() Meta
	GetOrders() (map[float64]Order, error)
	GetLastTradePriceForPair(buy, sell string) float64
}
