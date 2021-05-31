package main

import (
	"context"
	"encoding/json"
	"flag"
	// "fmt"
	sdk "invest-openapi-go-sdk"
	"io/ioutil"
	"log"
	"os"
	"time"
)

var token = flag.String("token", "", "my token")
var production = true
var endDayClosePosition = false
var StopLossPerc = 0.5
var StopLossCurrent float64 = 0.0
var hours, minutes, _ = time.Now().Clock()

func main() {
	log.Println("Starting bot")
	cleanState()
	for {
		rest()
		time.Sleep(10 * time.Second)
	}

}

func cleanState() {
	os.RemoveAll("./state/")
	os.MkdirAll("./state/", 0700)
}

func rest() {
	client := sdk.NewRestClient(*token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	positions, err := client.PositionsPortfolio(ctx, sdk.DefaultAccount)
	if err != nil {
		log.Fatalln(err)
	}

	for i := range positions {

		orderbook, err := client.Orderbook(ctx, 1, positions[i].FIGI)
		if err != nil {
			log.Fatalln(err)
		}

		// Close positions on end of the day
		if positions[i].Ticker != string("USD000UTSTOM") && (positions[i].AveragePositionPrice.Currency == "RUB" && hours == 00 && minutes == 50) || (positions[i].AveragePositionPrice.Currency == "USD" && hours == 04 && minutes == 40) {
			err := os.Remove("./state/short-" + positions[i].Ticker + ".json")
			if err != nil {
				log.Fatalln(err)
			}

			if production == true && endDayClosePosition == true {
				_, err := client.MarketOrder(ctx, sdk.DefaultAccount, positions[i].FIGI, positions[i].Lots, sdk.BUY)
				if err != nil {
					log.Fatalln(err)
				}

				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				log.Println("Close position", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
			} else {
				log.Println("Time to close position", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
			}

		} else if positions[i].Lots > 0 && positions[i].Ticker != string("USD000UTSTOM") {
			StopLossCalculated := orderbook.LastPrice - orderbook.LastPrice/100*StopLossPerc

			if _, err := os.Stat("./state/long-" + positions[i].Ticker + ".json"); os.IsNotExist(err) {
				file, _ := json.MarshalIndent(StopLossCalculated, "", "")
				_ = ioutil.WriteFile("./state/long-"+positions[i].Ticker+".json", file, 0644)
			}
			file, _ := ioutil.ReadFile("./state/long-" + positions[i].Ticker + ".json")
			_ = json.Unmarshal([]byte(file), &StopLossCurrent)

			if StopLossCurrent <= orderbook.LastPrice {
				if StopLossCurrent < StopLossCalculated {
					StopLossCurrent := StopLossCalculated
					file, _ := json.MarshalIndent(StopLossCurrent, "", "")
					_ = ioutil.WriteFile("./state/long-"+positions[i].Ticker+".json", file, 0644)

					log.Println("Move stop-loss order forward for", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
				}

			} else {
				err := os.Remove("./state/long-" + positions[i].Ticker + ".json")
				if err != nil {
					log.Fatalln(err)
				}
				if production == true {
					_, err := client.MarketOrder(ctx, sdk.DefaultAccount, positions[i].FIGI, positions[i].Lots, sdk.SELL)
					if err != nil {
						log.Fatalln(err)
					}

					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					log.Println("Close position", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
				} else {
					log.Println("Time to close position", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
				}
			}

		} else if positions[i].Lots < 0 && positions[i].Ticker != string("USD000UTSTOM") {
			StopLossCalculated := orderbook.LastPrice + orderbook.LastPrice/100*StopLossPerc

			if _, err := os.Stat("./state/short-" + positions[i].Ticker + ".json"); os.IsNotExist(err) {
				file, _ := json.MarshalIndent(StopLossCalculated, "", "")
				_ = ioutil.WriteFile("./state/short-"+positions[i].Ticker+".json", file, 0644)
			}
			file, _ := ioutil.ReadFile("./state/short-" + positions[i].Ticker + ".json")
			_ = json.Unmarshal([]byte(file), &StopLossCurrent)

			if StopLossCurrent > orderbook.LastPrice {
				if StopLossCurrent > StopLossCalculated {
					StopLossCurrent := StopLossCalculated
					file, _ := json.MarshalIndent(StopLossCurrent, "", "")
					_ = ioutil.WriteFile("./state/short-"+positions[i].Ticker+".json", file, 0644)

					log.Println("Move stop-loss order forward for", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
				}

			} else {
				err := os.Remove("./state/short-" + positions[i].Ticker + ".json")
				if err != nil {
					log.Fatalln(err)
				}

				if production == true {
					_, err := client.MarketOrder(ctx, sdk.DefaultAccount, positions[i].FIGI, positions[i].Lots, sdk.BUY)
					if err != nil {
						log.Fatalln(err)
					}

					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					log.Println("Close position", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
				} else {
					log.Println("Time to close position", positions[i].Lots, positions[i].Ticker, orderbook.LastPrice, "/", StopLossCurrent)
				}
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
}
