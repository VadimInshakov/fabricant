package broker

import "github.com/vadiminshakov/exmo"

type Fabricator interface {
	Sell(string, string, float64, float64) string
	MarketBuy(string, string, float64) string
	Buy(string, string, float64, float64) string
	WaitOrdersExecute()
	WhatICanBuy(string, string) (float64, error)
	WhatICanSell(string) float64
	Monitor()
	WaitForBuy(string, string, float64) string
	Save(float64, Order) error
	Delete(float64)
	GetConfig() Config
	GetTimers() Timers
	GetApi() *exmo.Exmo
	GetMeta() Meta
	SetMetaSelled(float64)
	GetOrders() (map[float64]Order, error)
	GetLastTradePriceForPair(string, string) float64
	GetOrderPrice(string) (float64, error)
}
