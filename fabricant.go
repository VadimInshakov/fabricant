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

package main

import (
	"flag"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/vadiminshakov/fabricant/broker"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func Start(fab broker.Fabricator, buy, sell string, withfunds bool) {

	conf := fab.GetConfig()

	orders, err := fab.GetOrders()
	if err != nil {
		panic(err)
	}
	config := fab.GetConfig()
	api := fab.GetApi()
	meta := fab.GetMeta()

	minPriceBigFloat := decimal.NewFromFloat(config.MinPrice)
	maxPriceBigFloat := decimal.NewFromFloat(config.MaxPrice)
	GapBigFloat := decimal.NewFromFloat(config.Gap)

	//date := time.Date(2019, 10, 4, 0, 0, 0, 0, time.UTC)
	//subdate := 10 * time.Hour
	//
	//resultWalletHistory, err := api.GetWalletHistory(date.Truncate(subdate))
	//
	//if err != nil {
	//	fmt.Errorf("api error: %s\n", err)
	//} else {
	//	fmt.Println(resultWalletHistory)
	//	for k, v := range resultWalletHistory {
	//		fmt.Println(k, v)
	//		if k == "history" {
	//			for key, val := range v.([]interface{}) {
	//				fmt.Println(key, val)
	//			}
	//		}
	//	}
	//}

	var lastPrice float64
	if withfunds {
		lastPrice = fab.GetLastTradePriceForPair(buy, sell)
		fmt.Printf("\nLast price: %f", lastPrice)
	}

	duration, err := time.ParseDuration(conf.Timers.PollInterval)
	if err != nil {
		panic(fmt.Sprintf("Can't parse duration, error: %s", err))
	}

	tick := time.NewTicker(duration)
	for {
		select {
		case <-tick.C:
			ticker, err := api.Ticker()
			if err != nil {
				fmt.Printf("api error: %s\n", err.Error())
			} else {
				for pair, pairvalue := range ticker {
					if pair == fmt.Sprintf("%s_%s", buy, sell) {
						for key, value := range pairvalue.(map[string]interface{}) {

							if key == "sell_price" {
								price, ok := value.(string)
								if !ok {
									fmt.Printf("interface to string converting error")
									continue
								}

								sellPrice, _ := decimal.NewFromString(price)

								if sellPrice.Cmp(minPriceBigFloat) > 0 && sellPrice.Cmp(maxPriceBigFloat) < 0 {

									sellPriceConverted, _ := sellPrice.Float64()

									orders, err = fab.GetOrders()
									if err != nil {
										panic(err)
									}

									// If you started with a closed transaction,
									// you need to determine the purchase price
									// by the amount that you can buy relative to already purchased amount
									haveOpenDeals := false
									if len(orders) > 0 {
										for _, v := range orders {
											if v.Closed {
												haveOpenDeals = true
											}
										}
									}

									if haveOpenDeals && !withfunds {
										for k, v := range orders {
											if v.Closed {
												// buy
												orderIdBuy := fab.WaitForBuy(buy, sell, k) // buy relative to already purchased amount
												fab.WaitOrdersExecute()
												fmt.Printf("Order %s closed", orderIdBuy)
											}
										}
									}

									if len(orders) == 0 && !withfunds {
										// calculate how much I can buy
										amountForBuy, err := fab.WhatICanBuy(buy, sell)
										if err != nil {
											fmt.Println(err)
											continue
										}

										//buy
										orderId := fab.MarketBuy(buy, sell, amountForBuy)
										fab.WaitOrdersExecute()
										fmt.Printf("Order %s closed", orderId)

										// get marketBuy order price
										buyedFor, err := fab.GetOrderPrice(orderId)
										if err != nil {
											panic(err)
										}

										//save order to redis or map
										fab.Save(sellPriceConverted, broker.Order{false, 0, buyedFor, amountForBuy})
										fmt.Printf("\nNew fund created with price: %.2f\n", sellPriceConverted)
									} else {
										for k, v := range orders {
											if !v.Closed { // "Closed" order is the order that sold

												kBigFloat := decimal.NewFromFloat(k)
												tmpDelta := sellPrice.Sub(GapBigFloat)

												if tmpDelta.Cmp(kBigFloat) > 0 {
													fee, _ := decimal.NewFromString("0.002")
													tmpamountForSell := fab.WhatICanSell(buy)
													amountForSell := decimal.NewFromFloat(tmpamountForSell)
													if amountForSell.Cmp(decimal.NewFromFloat(0.004)) == 1 {
														sellAmount := amountForSell.Mul(sellPrice)
														fee = sellAmount.Mul(fee)
														sellTotal := sellAmount.Sub(fee)

														buyedVolume := decimal.NewFromFloat(v.Volume)
														var buyedPrice decimal.Decimal
														if !withfunds {
															buyedPrice = decimal.NewFromFloat(k)
														} else {
															buyedPrice = decimal.NewFromFloat(lastPrice)
														}

														alreadyBuyedValue := buyedVolume.Mul(buyedPrice)

														// At the current price, is revenue greater than at the previous one?
														if sellTotal.Cmp(alreadyBuyedValue) > 0 {

															// sell
															sellPriceConverted, _ := sellPrice.Float64()
															amountForSellConverted, _ := amountForSell.Float64()

															if amountForSell.Cmp(meta.TRADELIMIT) >= 0 {
																fmt.Println("Sell with price ", sellPriceConverted)
																orderId := fab.Sell(buy, sell, amountForSellConverted, sellPriceConverted)
																if withfunds {
																	withfunds = false
																}
																if orderId != "" {

																	fab.WaitOrdersExecute()
																	fmt.Printf("Order %s closed", orderId)

																	fab.SetMetaSelled(k)
																	fab.Save(k, broker.Order{true, sellPriceConverted, k, amountForSellConverted})
																	fmt.Printf("\nFund %s selled for %f %s, amount %f", buy, sellPriceConverted, sell, amountForSellConverted)

																	// calculate again how much I can buy
																	amountForBuy, err := fab.WhatICanBuy(buy, sell)
																	if err != nil {
																		fmt.Println(err)
																		continue
																	}

																	// buy
																	orderIdBuy := fab.WaitForBuy(buy, sell, amountForBuy)
																	fab.WaitOrdersExecute()
																	fmt.Printf("Order %s closed", orderIdBuy)
																}
															}
														}
													}
												}
											}
										}
									}
								} else {
									fmt.Printf("\nLimit reached, sell price now: %f", sellPrice)
								}
							}
						}
					}
				}
			}
		}
	}
}

func main() {

	confpath := flag.String("config", "./config.yaml", "path to YAML config")
	withfunds := flag.Bool("withfunds", false, "start with crypto funds already buyed (true) or not (false)")
	flag.Parse()

	// read config
	data, err := ioutil.ReadFile(*confpath)
	if err != nil {
		log.Println("Reading file error: ")
		return
	}

	var globalConfig broker.Config
	err = yaml.Unmarshal([]byte(data), &globalConfig)
	if err != nil {
		log.Println("Unmarshalling error: ")
		return
	}

	fabricant := broker.NewFabricant(globalConfig)

	go Start(fabricant, "BTC", "RUB", *withfunds)
	go fabricant.Monitor()

	// just a stupid stub for Heroku deployment
	http.HandleFunc("/", stub)

	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("Can't find port for binding from $PORT, using default port 8080")
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))

}

func stub(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Fabricant bot")
}
