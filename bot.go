package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	sdk "invest-openapi-go-sdk"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var token = flag.String("token", "", "prod")
var tgToken = ""
var production = false
var dayTrader = true
var slPerc = 0.5
var slCurrent float64 = 0.0
var hours, minutes, _ = time.Now().Clock()

func main() {

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

func rmState(ticker string, strategy string) {
	err := os.Remove("./state/" + ticker + "." + strategy)
	if err != nil {
		log.Fatalln(err)
	}
}

func checkState(ticker string, sl float64, strategy string) {
	if _, err := os.Stat("./state/" + ticker + "." + strategy); os.IsNotExist(err) {
		file, _ := json.MarshalIndent(sl, "", "")
		_ = ioutil.WriteFile("./state/"+ticker+"."+strategy, file, 0644)

		msg := fmt.Sprintf("Set SL order for %s to %s", ticker, strconv.FormatFloat(sl, 'f', 2, 64))
		_, errtg := tg(msg)
		if errtg != nil {
			log.Fatalln(errtg)
		}
	}
}

func updateState(ticker string, sl float64, strategy string) {
	file, _ := json.MarshalIndent(sl, "", "")
	_ = ioutil.WriteFile("./state/"+ticker+"."+strategy, file, 0644)
	msg := fmt.Sprintf("Move SL order for %s to %s", ticker, strconv.FormatFloat(sl, 'f', 2, 64))
	_, errtg := tg(msg)
	if errtg != nil {
		log.Fatalln(errtg)
	}
}

func closePosition(figi string, ticker string, lots int, strategy string) {
	client := sdk.NewSandboxRestClient(*token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if strategy == "long" {
		_, err := client.MarketOrder(ctx, sdk.DefaultAccount, figi, lots, sdk.SELL)
		if err != nil {
			log.Fatalln(err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

	} else if strategy == "short" {
		_, err := client.MarketOrder(ctx, sdk.DefaultAccount, figi, lots*-1, sdk.BUY)
		if err != nil {
			log.Fatalln(err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	msg := fmt.Sprintf("Close position for %s lots of %s", strconv.Itoa(lots), ticker)
	_, errtg := tg(msg)
	if errtg != nil {
		log.Fatalln(errtg)
	}
}

func tg(msg string) (string, error) {

	_, err := http.PostForm("https://api.telegram.org/bot"+tgToken+"/sendMessage",
		url.Values{
			"chat_id": {"58232431"},
			"text":    {msg},
		})
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err != nil {
		log.Fatalln(err)
	}

	return "OK", nil
}

func rest() {
	client := sdk.NewRestClient(*token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := client.PositionsPortfolio(ctx, sdk.DefaultAccount) //positions
	if err != nil {
		log.Fatalln(err)
	}

	for i := range p { // close positions on end of the day

		if dayTrader == true &&
			(p[i].AveragePositionPrice.Currency == "RUB" && hours == 03 && minutes == 30) ||
			(p[i].AveragePositionPrice.Currency == "USD" && hours == 04 && minutes == 30) {
			if p[i].Lots > 0 && p[i].Ticker != string("USD000UTSTOM") {
				strategy := "long"
				if production == true {
					closePosition(p[i].FIGI, p[i].Ticker, p[i].Lots, strategy)
					rmState(p[i].Ticker, strategy)
				}

			} else if p[i].Lots < 0 && p[i].Ticker != string("USD000UTSTOM") {
				strategy := "short"
				if production == true {
					closePosition(p[i].FIGI, p[i].Ticker, p[i].Lots, strategy)
					rmState(p[i].Ticker, strategy)
				}

			}
		}
	}

	for i := range p { // stop-loss logic
		// long
		if p[i].Lots > 0 && p[i].Ticker != string("USD000UTSTOM") {
			strategy := "long"

			ob, err := client.Orderbook(ctx, 1, p[i].FIGI)
			if err != nil {
				log.Fatalln(err)
			}
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			slCalculated := ob.LastPrice - ob.LastPrice/100*slPerc
			checkState(p[i].Ticker, slCalculated, strategy)

			file, err := ioutil.ReadFile("./state/" + p[i].Ticker + "." + strategy)
			_ = json.Unmarshal([]byte(file), &slCurrent)
			if err != nil {
				log.Fatalln(err)
			}

			if slCurrent < slCalculated {
				if slCurrent < slCalculated {
					log.Println("slcur", slCalculated)
					updateState(p[i].Ticker, slCalculated, strategy)
				}

			} else {
				if production == true {
					closePosition(p[i].FIGI, p[i].Ticker, p[i].Lots, strategy)
					rmState(p[i].Ticker, strategy)
				}
			}

		} else {
			// short
			if p[i].Lots < 0 && p[i].Ticker != string("USD000UTSTOM") {
				strategy := "short"

				ob, err := client.Orderbook(ctx, 1, p[i].FIGI)
				if err != nil {
					log.Fatalln(err)
				}
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				slCalculated := ob.LastPrice + ob.LastPrice/100*slPerc
				checkState(p[i].Ticker, slCalculated, strategy)

				file, _ := ioutil.ReadFile("./state/" + p[i].Ticker + "." + strategy)
				_ = json.Unmarshal([]byte(file), &slCurrent)
				if err != nil {
					log.Fatalln(err)
				}

				if slCurrent > ob.LastPrice {
					if slCurrent > slCalculated {
						log.Println("slcur", slCalculated)
						updateState(p[i].Ticker, slCalculated, strategy)
					}

				} else {
					if production == true {
						closePosition(p[i].FIGI, p[i].Ticker, p[i].Lots, strategy)
						rmState(p[i].Ticker, strategy)
					}
				}
			}
		}
	}
}
