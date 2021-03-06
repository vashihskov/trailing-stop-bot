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
var dayTrader = false
var slPerc = 0.5
var hours, minutes, _ = time.Now().Clock()
var excludePositions = []string{"USD000UTSTOM", "SPCE"}
var obChannel = make(chan interface{})

func main() {
	cleanState()
	for {
		stopLoss()
		time.Sleep(5 * time.Second)
	}
}

func cleanState() {
	os.RemoveAll("./state/")
	os.MkdirAll("./state/", 0700)
}

func getPositions() []sdk.PositionBalance {
	client := sdk.NewRestClient(*token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := client.PositionsPortfolio(ctx, sdk.DefaultAccount) //positions
	if err != nil {
		log.Fatalln(err)
	}

	return p
}

func positionExclude(ticker string) bool {
	for i := range excludePositions {
		if ticker == excludePositions[i] {
			return true
		}
	}
	return false
}

func getPrice(figi string) float64 {
	client := sdk.NewRestClient(*token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ob, err := client.Orderbook(ctx, 1, figi)
	if err != nil {
		log.Fatalln(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return ob.LastPrice
}

func checkState(ticker, strategy string, sl float64, lots int) float64 {
	if _, err := os.Stat("./state/" + ticker + "." + strategy); os.IsNotExist(err) {
		file, _ := json.MarshalIndent(sl, "", "")
		_ = ioutil.WriteFile("./state/"+ticker+"."+strategy, file, 0644)

		file, err := ioutil.ReadFile("./state/" + ticker + "." + strategy)
		_ = json.Unmarshal([]byte(file), &sl)
		if err != nil {
			log.Fatalln(err)
		}

		msg := fmt.Sprintf("Set SL order for %d lots of %s to %s", lots, ticker, strconv.FormatFloat(sl, 'f', 2, 64))
		_, errtg := tg(msg)
		if errtg != nil {
			log.Fatalln(errtg)
		}
	} else {
		file, err := ioutil.ReadFile("./state/" + ticker + "." + strategy)
		_ = json.Unmarshal([]byte(file), &sl)
		if err != nil {
			log.Fatalln(err)
		}
	}

	return sl
}

func updateState(ticker, strategy string, sl float64, lots int) {
	file, _ := json.MarshalIndent(sl, "", "")
	_ = ioutil.WriteFile("./state/"+ticker+"."+strategy, file, 0644)

	msg := fmt.Sprintf("Update SL order for %d lots of %s to %s", lots, ticker, strconv.FormatFloat(sl, 'f', 2, 64))
	_, errtg := tg(msg)
	if errtg != nil {
		log.Fatalln(errtg)
	}
}

func closePosition(figi, ticker, strategy string, lots int) {
	if production == true {
		client := sdk.NewRestClient(*token)
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
	}

	err := os.Remove("./state/" + ticker + "." + strategy)
	if err != nil {
		log.Fatalln(err)
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

func stopLoss() {

	p := getPositions()

	for i := range p { // close positions on end of the day

		if dayTrader == true &&
			(p[i].AveragePositionPrice.Currency == "RUB" && hours == 03 && minutes == 30) ||
			(p[i].AveragePositionPrice.Currency == "USD" && hours == 04 && minutes == 30) {
			if p[i].Lots > 0 && positionExclude(p[i].Ticker) == false {
				strategy := "long"
				closePosition(p[i].FIGI, p[i].Ticker, strategy, p[i].Lots)

			} else if p[i].Lots < 0 && positionExclude(p[i].Ticker) == false {
				strategy := "short"
				closePosition(p[i].FIGI, p[i].Ticker, strategy, p[i].Lots)
			}
		}
	}

	for i := range p { // SL logic
		// long
		if p[i].Lots > 0 && positionExclude(p[i].Ticker) == false {
			strategy := "long"
			lastPrice := getPrice(p[i].FIGI)
			slCalculated := lastPrice - lastPrice/100*slPerc
			slCurrent := checkState(p[i].Ticker, strategy, slCalculated, p[i].Lots)

			if slCurrent < lastPrice {
				if slCurrent < slCalculated {
					updateState(p[i].Ticker, strategy, slCalculated, p[i].Lots)
				}

			} else {
				closePosition(p[i].FIGI, p[i].Ticker, strategy, p[i].Lots)
			}

		} else {
			// short
			if p[i].Lots < 0 && positionExclude(p[i].Ticker) == false {
				strategy := "short"
				lastPrice := getPrice(p[i].FIGI)
				slCalculated := lastPrice + lastPrice/100*slPerc
				slCurrent := checkState(p[i].Ticker, strategy, slCalculated, p[i].Lots)

				if slCurrent > lastPrice {
					if slCurrent > slCalculated {
						updateState(p[i].Ticker, strategy, slCalculated, p[i].Lots)
					}

				} else {
					closePosition(p[i].FIGI, p[i].Ticker, strategy, p[i].Lots)
				}
			}
		}
	}
}
