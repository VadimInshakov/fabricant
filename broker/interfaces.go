/*
   Copyright 2019 Vadim Inshakov

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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
