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

	orders, err := fab.GetOrders()
	if err != nil {
		panic(err)
	}
	config := fab.GetConfig()
	timers := fab.GetTimers()
	api := fab.GetApi()
	meta := fab.GetMeta()

	minPriceBigFloat := decimal.NewFromFloat(config.MinPrice)
	maxPriceBigFloat := decimal.NewFromFloat(config.MaxPrice)
	GapBigFloat := decimal.NewFromFloat(config.Gap)

	date := time.Date(2019, 10, 4, 0, 0, 0, 0, time.UTC)
	subdate := 10 * time.Hour

	resultWalletHistory, err := api.GetWalletHistory(date.Truncate(subdate))

	if err != nil {
		fmt.Errorf("api error: %s\n", err)
	} else {
		fmt.Println(resultWalletHistory)
		for k, v := range resultWalletHistory {
			fmt.Println(k, v)
			if k == "history" {
				for key, val := range v.([]interface{}) {
					fmt.Println(key, val)
				}
			}
		}
	}

	var lastPrice float64
	if withfunds {
		lastPrice = fab.GetLastTradePriceForPair(buy, sell)
		fmt.Printf("\nLast price: %f", lastPrice)
	}

	tick := time.NewTicker(timers.POLLINTERVAL)
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

									amountForBuy, err := fab.WhatICanBuy(buy, sell)
									if err != nil {
										fmt.Println(err)
										continue
									}

									sellPriceConverted, _ := sellPrice.Float64()

									orders, err = fab.GetOrders()
									if err != nil {
										panic(err)
									}

									if len(orders) == 0 && !withfunds {
										//buy
										orderId := fab.MarketBuy(buy, sell, amountForBuy)
										fab.WaitOrdersExecute()
										fmt.Printf("Order %s closed", orderId)
										fab.Save(sellPriceConverted, broker.Order{false, 0, 0, amountForBuy})
										fmt.Printf("\nNew fund created with price: %.2f\n", sellPriceConverted)
									} else {

										for k, v := range orders {

											if !v.Closed {

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

																	meta.SELLEDNOW = k
																	fab.Save(k, broker.Order{true, sellPriceConverted, k, amountForSellConverted})
																	fmt.Printf("\nFund %s selled for %f %s, amount %f", buy, sellPriceConverted, sell, amountForSellConverted)

																	// buy
																	orderIdBuy := fab.WaitForBuy(buy, sell, k)
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
