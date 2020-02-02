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

import (
	"github.com/go-redis/redis"
	"github.com/shopspring/decimal"
	"github.com/vadiminshakov/exmo"
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
	Meta
	Db *redis.Client
}

type Timers struct {
	PollInterval string `yaml:"pollinterval"`
	WaitForBuy   string `yaml:"waitforbuy"`
	OrdersCheck  string `yaml:"orderscheck"`
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
	Timers
}
