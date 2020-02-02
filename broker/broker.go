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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/shopspring/decimal"
	"github.com/vadiminshakov/exmo"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func NewFabricant(conf Config) *Fabricant {
	var client *redis.Client
	ordersMap := make(map[float64]Order)
	ch := make(chan map[float64]float64)
	api := exmo.Api(os.Getenv("EXMO_PUBLIC"), os.Getenv("EXMO_SECRET"))
	tradelimit := decimal.NewFromFloat(0.0001)

	if conf.UseRedis {
		client = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", conf.DbAddr, conf.DbPort),
			Password: conf.DbPass, // no password set
			DB:       conf.DbNum,  // use default DB
		})

		_, err := client.Ping().Result()
		if err != nil {
			log.Fatalf("\nCan't connect to Redis, %s", err)
		}

	}
	return &Fabricant{ch, conf, ordersMap, &api, Timers{POLLINTERVAL: 1 * time.Second, WAITFORBUY: 800 * time.Millisecond, ORDERSCHECK: 700 * time.Millisecond}, Meta{0, tradelimit}, client}
}

func (fab *Fabricant) Sell(sell, buy string, volume, price float64) string {
	var orderId string
	orderSell, err := fab.Api.Sell(fmt.Sprintf("%s_%s", sell, buy), fmt.Sprintf("%f", volume), fmt.Sprintf("%f", price))
	for err != nil && strings.Contains(err.Error(), "Insufficient funds") {
		fmt.Printf("\nInsufficient funds: \nvolume %f replaced with ", volume)
		volume = volume - volume*0.001
		fmt.Printf("%f\n", volume)
		orderSell, err = fab.Api.Sell(fmt.Sprintf("%s_%s", sell, buy), fmt.Sprintf("%f", volume), fmt.Sprintf("%f", price))
	}
	fmt.Println("Creating order")
	for key, value := range orderSell {
		if key == "result" && value != true {
			fmt.Println("\nError")
		}
		if key == "error" && value != "" {
			fmt.Println(value)
		}
		if key == "order_id" && value != nil {
			val := strconv.Itoa(int(value.(float64)))
			orderId = val
		}
	}
	return orderId
}
func (fab *Fabricant) MarketBuy(buy, sell string, volume float64) string {
	var orderId string
	order, err := fab.Api.MarketBuy(fmt.Sprintf("%s_%s", buy, sell), fmt.Sprintf("%f", volume))
	for err != nil && strings.Contains(err.Error(), "Insufficient funds") {
		fmt.Printf("\nInsufficient funds: \nvolume %f replaced with ")
		volume = volume - volume*0.1
		fmt.Sprintf("%f", volume)
		order, err = fab.Api.MarketBuy(fmt.Sprintf("%s_%s", buy, sell), fmt.Sprintf("%f", volume))
	}

	if err != nil {
		fmt.Printf("api error: %s\n", err.Error())
	}
	fmt.Println("Creating order")
	for key, value := range order {
		if key == "result" && value != true {
			fmt.Println("\nError")
			break
		}
		if key == "error" && value != "" {
			fmt.Println(value)
			break
		}
		if key == "order_id" && value != nil {
			fmt.Printf("Order id: %d\n", int(value.(float64)))
			val := strconv.Itoa(int(value.(float64)))
			orderId = val
			break
		}
	}
	return orderId
}

func (fab *Fabricant) Buy(buy, sell string, volume, price float64) string {
	var orderId string
	order, err := fab.Api.Buy(fmt.Sprintf("%s_%s", buy, sell), fmt.Sprintf("%f", volume), fmt.Sprintf("%f", price))
	for err != nil && strings.Contains(err.Error(), "Insufficient funds") {
		fmt.Printf("\nInsufficient funds: \nvolume %f replaced with ")
		volume = volume - volume*0.1
		fmt.Sprintf("%f", volume)
		order, err = fab.Api.Buy(fmt.Sprintf("%s_%s", buy, sell), fmt.Sprintf("%f", volume), fmt.Sprintf("%f", price))

	}
	fmt.Println("Creating order")
	for key, value := range order {
		if key == "result" && value != true {
			fmt.Println("\nError")
			break
		}
		if key == "error" && value != "" {
			fmt.Println(value)
			break
		}
		if key == "order_id" && value != nil {
			fmt.Printf("Order id: %d\n", int(value.(float64)))
			val := strconv.Itoa(int(value.(float64)))
			orderId = val
			break
		}
	}
	return orderId
}

func (fab *Fabricant) WaitOrdersExecute() {
	tick := time.NewTicker(fab.Timers.ORDERSCHECK)
	for {
		select {
		case <-tick.C:
			resultUserOpenOrders, err := fab.Api.GetUserOpenOrders()
			if err != nil {
				fmt.Errorf("api error: %s\n", err.Error())
			}
			if len(resultUserOpenOrders) == 0 {
				tick.Stop()
				return
			}
		}
	}
}

func (fab *Fabricant) WhatICanBuy(buy, sell string) (float64, error) {

	var IHave decimal.Decimal
	var buyPrice decimal.Decimal

	// find free funds
	resultUserInfo, err := fab.Api.GetUserInfo()

	if err != nil {
		fmt.Printf("api error: %s\n", err.Error())
	} else {
		for key, value := range resultUserInfo {
			if key == "balances" {
				for k, v := range value.(map[string]interface{}) {
					if k == sell {
						IHave, _ = decimal.NewFromString(v.(string))
					}
				}
			}
		}
	}

	// get market buy price
	ticker, err := fab.Api.Ticker()
	if err != nil {
		return 0, err
	} else {
		for pair, pairvalue := range ticker {
			if pair == fmt.Sprintf("%s_%s", buy, sell) {
				for key, value := range pairvalue.(map[string]interface{}) {

					if key == "buy_price" {
						buyPrice, _ = decimal.NewFromString(value.(string))
					}
				}
			}
		}
	}

	// calculate amount that I can buy for my funds
	var resultConverted float64
	if buyPrice.Cmp(decimal.NewFromFloat(0)) == 1 {
		result := IHave.Div(buyPrice)
		resultConverted, _ = result.Float64()
	} else {
		return 0, errors.New(fmt.Sprintf("Got invalid buy price for %s: %f %s", buy, buyPrice, sell))
	}
	return resultConverted, nil
}

func (fab *Fabricant) WhatICanSell(currency string) float64 {
	// find free funds
	var IHave float64
	resultUserInfo, err := fab.Api.GetUserInfo()
	if err != nil {
		fmt.Printf("api error: %s\n", err.Error())
	} else {
		for key, value := range resultUserInfo {
			if key == "balances" {

				for k, v := range value.(map[string]interface{}) {
					if k == currency {
						IHave, err := strconv.ParseFloat(v.(string), 64)
						if err != nil {
							fmt.Printf("parsing float64 error: %s\n", err.Error())
						}
						return IHave
					}
				}
			}

		}

	}
	return IHave
}

func (fab *Fabricant) Monitor() {
	msg := make(map[float64]float64)
	for {
		select {
		case msg = <-fab.Ch:
			for k, v := range msg {
				was := decimal.NewFromFloat(k)
				became := decimal.NewFromFloat(v)
				delta := became.Sub(was)
				fmt.Printf("\n + %d crypto", delta)
				fab.Delete(k)
				delete(msg, k)
			}
		}
	}
}

func (fab *Fabricant) WaitForBuy(buy, sell string, priceAlreadyBuyed float64) string {

	// listen for market prices
	tick := time.NewTicker(fab.Timers.WAITFORBUY)
	for {
		select {
		case <-tick.C:
			ticker, err := fab.Api.Ticker()
			if err != nil {
				fmt.Printf("api error: %s\n", err.Error())
			} else {
				for pair, pairvalue := range ticker {
					if pair == fmt.Sprintf("%s_%s", buy, sell) {
						for key, value := range pairvalue.(map[string]interface{}) {

							if key == "buy_price" {
								// buyPrice - price for asset right now, priceBigFloat - price for which asset buyed
								floatValue, err := strconv.ParseFloat(value.(string), 64)
								if err != nil {
									fmt.Println("conversion interface to float error:", err)
								}

								// Can I buy more crypto now than last time?
								amountForBuyNow, err := fab.WhatICanBuy(buy, sell)
								if err != nil {
									fmt.Println(err)
									continue
								}

								fmt.Println("priceAlreadyBuyed", priceAlreadyBuyed)
								alreadyBuyedOrder, err := fab.Get(priceAlreadyBuyed)
								if err != nil {
									panic(err)
								}

								amountAlreadyBuyedDecimal := decimal.NewFromFloat(alreadyBuyedOrder.Volume)
								amountForBuyNowDecimal := decimal.NewFromFloat(amountForBuyNow)

								if amountForBuyNowDecimal.Cmp(amountAlreadyBuyedDecimal) != 1 {
									continue
								}
								var checkOrderExist Order
								if fab.Conf.UseRedis {
									val, err := fab.Get(floatValue)
									if err != nil {
										checkOrderExist = Order{}
									} else {
										checkOrderExist = val
									}
								} else {
									checkOrderExist = fab.Orders[floatValue]
								}
								if (checkOrderExist == Order{}) {

									//buy
									orderId := fab.Buy(buy, sell, amountForBuyNow, floatValue)
									err = fab.Save(floatValue, Order{false, 0, floatValue, amountForBuyNow})
									if err != nil {
										panic(err)
									}
									fmt.Printf("\nFund %s buyed for %f %s, amount %f", buy, floatValue, sell, amountForBuyNow)

									tmpstore, err := fab.Get(fab.SELLEDNOW)
									if err != nil {
										panic(err)
									}
									err = fab.Save(fab.SELLEDNOW, Order{tmpstore.Closed, tmpstore.SellPrice, floatValue, tmpstore.Volume})
									if err != nil {
										panic(err)
									}
									newMessage := make(map[float64]float64)
									newMessage[alreadyBuyedOrder.Volume] = amountForBuyNow
									fab.Ch <- newMessage
									tick.Stop()

									return orderId
								}
							}
						}
					}
				}
			}
		}
	}
}

func (fab *Fabricant) Get(key float64) (Order, error) {
	if fab.Conf.UseRedis {
		val, err := fab.Db.Get(fmt.Sprintf("%f", key)).Result()
		if err != nil {
			return Order{}, err
		}

		order := &Order{}
		err = json.Unmarshal([]byte(val), order)
		if err != nil {
			return Order{}, err
		}

		return *order, nil
	} else {
		return fab.Orders[key], nil
	}
}

func (fab *Fabricant) Save(key float64, value Order) error {
	if fab.Conf.UseRedis {
		bytesJson, err := json.Marshal(value)
		if err != nil {
			return err
		}

		err = fab.Db.Set(fmt.Sprintf("%f", key), bytesJson, 0).Err()
		if err != nil {
			return err
		}
		return nil
	} else {
		fab.Orders[key] = value
		return nil
	}
}

func (fab *Fabricant) Delete(key float64) {
	if fab.Conf.UseRedis {
		err := fab.Db.Del(fmt.Sprintf("%f", key)).Err()
		if err != nil {
			panic(err)
		}
	} else {
		delete(fab.Orders, key)
	}
}

func (fab *Fabricant) GetLastTradePriceForPair(buy, sell string) float64 {
	var lastPrice float64
	if fab.Conf.UseRedis {
		val, err := fab.Db.Do("KEYS", "*").Result()
		if err != nil {
			panic(err)
		}

		for _, value := range val.([]interface{}) {
			val, err := fab.Db.Get(value.(string)).Result()
			if err != nil {
				panic(err)
			}
			order := &Order{}
			err = json.Unmarshal([]byte(val), order)
			if err != nil {
				panic(err)
			}

			if !order.Closed {
				return order.BuyPrice
			}
		}
	} else {
		usertrades, err1 := fab.Api.GetUserTrades(fmt.Sprintf("%s_%s", buy, sell), 0, 10000)
		if err1 != nil {
			fmt.Printf("api error: %s\n", err1.Error())
		} else {
			var tradeDates []float64
			for pair, val := range usertrades {
				if pair == fmt.Sprintf("%s_%s", buy, sell) {
					for _, interfacevalue := range val.([]interface{}) {
						for k, v := range interfacevalue.(map[string]interface{}) {
							if k == "date" {
								tradeDates = append(tradeDates, v.(float64))
							}
						}
					}
				}
			}
			sort.Float64s(tradeDates)
			for pair, val := range usertrades {
				if pair == fmt.Sprintf("%s_%s", buy, sell) {

					for _, interfacevalue := range val.([]interface{}) {
						mapWithTrades := interfacevalue.(map[string]interface{})
						for k, v := range mapWithTrades {
							if k == "date" {
								checkvalue := decimal.NewFromFloat(v.(float64))
								userTrade := decimal.NewFromFloat(tradeDates[len(tradeDates)-1])
								if checkvalue.Cmp(userTrade) == 0 {
									convertedPrice, err := strconv.ParseFloat(mapWithTrades["price"].(string), 64)
									if err != nil {
										fmt.Printf("String converting error")
										continue
									}
									lastPrice = convertedPrice
								}
							}
						}
					}
				}
			}
		}
	}
	return lastPrice
}

func (fab *Fabricant) GetConfig() Config {
	return fab.Conf
}

func (fab *Fabricant) GetTimers() Timers {
	return fab.Timers
}

func (fab *Fabricant) GetApi() *exmo.Exmo {
	return fab.Api
}

func (fab *Fabricant) GetMeta() Meta {
	return fab.Meta
}

func (fab *Fabricant) SetMetaSelled(price float64) {
	fab.SELLEDNOW = price
}

func (fab *Fabricant) GetOrderPrice(orderId string) (float64, error) {

	var priceConverted float64
	var err error
	resultUserOpenOrders, err := fab.Api.GetUserOpenOrders()
	if err != nil {
		fmt.Errorf("api error: %s\n", err)
	} else {
		for _, v := range resultUserOpenOrders {
			if v != nil {
				for _, val := range v.([]interface{}) {
					for key, value := range val.(map[string]interface{}) {
						if key == "order_id" {
							if value.(string) == orderId {
								price := val.(map[string]interface{})["price"]
								priceConverted, err := strconv.ParseFloat(price.(string), 64)
								if err != nil {
									return 0, err
								}
								return priceConverted, nil
							}
						}
					}
				}
			}
		}
	}
	return priceConverted, err
}

func (fab *Fabricant) GetOrders() (map[float64]Order, error) {
	if fab.Conf.UseRedis {
		var result = make(map[float64]Order)

		keys, err := fab.Db.Do("KEYS", "*").Result()
		if err != nil {
			return nil, err
		}

		for _, v := range keys.([]interface{}) {
			val, err := fab.Db.Get(v.(string)).Result()
			if err != nil {
				return nil, err
			}

			order := &Order{}
			err = json.Unmarshal([]byte(val), order)
			if err != nil {
				return nil, err
			}

			floatvalue, err := strconv.ParseFloat(v.(string), 64)
			if err != nil {
				return nil, err
			}
			result[floatvalue] = *order
		}
		return result, nil
	} else {
		return fab.Orders, nil
	}
}
