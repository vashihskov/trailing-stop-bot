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

func main() {
	log.Println("Starting bot")
	for {
		rest()
		time.Sleep(10 * time.Second)
	}

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

		var StopLossPerc = 0.5
		var StopLossCurrent float64 = 0.0

		orderbook, err := client.Orderbook(ctx, 1, positions[i].FIGI)
		if err != nil {
			log.Fatalln(err)
		}

		if positions[i].Lots > 0 {
			StopLossCalculated := orderbook.LastPrice - orderbook.LastPrice/100*StopLossPerc

			if _, err := os.Stat(positions[i].FIGI); os.IsNotExist(err) {
				file, _ := json.MarshalIndent(StopLossCalculated, "", "")
				_ = ioutil.WriteFile(positions[i].FIGI, file, 0644)
			}
			file, _ := ioutil.ReadFile(positions[i].FIGI)
			_ = json.Unmarshal([]byte(file), &StopLossCurrent)

			if StopLossCurrent <= orderbook.LastPrice {
				if StopLossCurrent < StopLossCalculated {
					StopLossCurrent := StopLossCalculated
					file, _ := json.MarshalIndent(StopLossCurrent, "", "")
					_ = ioutil.WriteFile(positions[i].FIGI, file, 0644)

					log.Println("Move stop-loss order forward for", orderbook.FIGI, orderbook.LastPrice, "/", StopLossCurrent, "/", positions[i].Lots)
				}

			} else {
				err := os.Remove(positions[i].FIGI)
				if err != nil {
					log.Fatalln(err)
				}

				log.Println("Close position", orderbook.FIGI, orderbook.LastPrice, "/", StopLossCurrent, "/", positions[i].Lots)
			}

		} else {
			StopLossCalculated := orderbook.LastPrice + orderbook.LastPrice/100*StopLossPerc

			if _, err := os.Stat(positions[i].FIGI); os.IsNotExist(err) {
				file, _ := json.MarshalIndent(StopLossCalculated, "", "")
				_ = ioutil.WriteFile(positions[i].FIGI, file, 0644)
			}
			file, _ := ioutil.ReadFile(positions[i].FIGI)
			_ = json.Unmarshal([]byte(file), &StopLossCurrent)

			if StopLossCurrent > orderbook.LastPrice {
				if StopLossCurrent > StopLossCalculated {
					StopLossCurrent := StopLossCalculated
					file, _ := json.MarshalIndent(StopLossCurrent, "", "")
					_ = ioutil.WriteFile(positions[i].FIGI, file, 0644)

					log.Println("Move stop-loss order forward for", orderbook.FIGI, orderbook.LastPrice, "/", StopLossCurrent, "/", positions[i].Lots)
				}

			} else {
				err := os.Remove(positions[i].FIGI)
				if err != nil {
					log.Fatalln(err)
				}

				log.Println("Close position", orderbook.FIGI, orderbook.LastPrice, "/", StopLossCurrent, "/", positions[i].Lots)
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
}
