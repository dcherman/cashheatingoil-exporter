package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	lowestCashPrice = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oil_lowest_price_cash",
		Help: "The lowest cash price per gallon available in USD",
	})

	lowestCreditPrice = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oil_lowest_price_credit",
		Help: "The lowest credit price per gallon available in USD",
	})
)

func getLowestPriceFromSelector(doc *goquery.Document, selector string) (float64, error) {
	selection := doc.Find(selector)

	if selection.Length() == 0 {
		return 0, fmt.Errorf("failed to find selector in document: %v", selector)
	}

	var prices []float64

	var err error

	selection.Each(func(i int, s *goquery.Selection) {
		var price float64

		contents := s.Text()
		price, err = strconv.ParseFloat(contents[1:], 32)

		if err != nil {
			logrus.Errorf("failed to convert text to price: %v", err)
			return
		}

		price = math.Round(price*100) / 100
		prices = append(prices, price)
	})

	if err != nil {
		return 0, err
	}

	min := math.MaxFloat64

	for _, price := range prices {
		min = math.Min(min, price)
	}

	return min, nil
}

func recordMetrics(scrapeURL string) {
	logrus.Debug("scraping url")
	response, err := http.Get(scrapeURL)

	if err != nil {
		logrus.Errorf("failed to make http request: %v", err)
		return
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		logrus.Errorf("expected status code 200, got %d", response.StatusCode)
		return
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)

	if err != nil {
		logrus.Errorf("failed to create reader from response body: %v", err)
		return
	}

	cashPrice, err := getLowestPriceFromSelector(doc, "table.paywithcash tr:nth-child(3) td:last-child")

	if err != nil {
		logrus.Errorf("failed to find low cash price: %v", err)
	} else {
		logrus.Infof("low cash price found: %v", cashPrice)
		lowestCashPrice.Set(cashPrice)
	}

	creditPrice, err := getLowestPriceFromSelector(doc, "table.paybycredit tr:nth-child(3) td:last-child")

	if err != nil {
		logrus.Errorf("failed to find low credit price: %v", err)
	} else {
		logrus.Infof("low credit price found: %v", creditPrice)
		lowestCreditPrice.Set(creditPrice)
	}
}

func main() {
	port := pflag.Int("port", 8000, "The port to listen on")
	scrapeURL := pflag.String("scrape-url", "", "The cash heating oil URL to scrape")
	scrapeInterval := pflag.Duration("scrape-interval", time.Hour, "The interval at which to scrape the URL")
	metricsPath := pflag.String("metrics-path", "/metrics", "The path to serve metrics on")

	pflag.Parse()

	if *scrapeURL == "" {
		panic("--scrapeURL is a required flag")
	}

	recordMetrics(*scrapeURL)

	go func() {
		for {
			time.Sleep(*scrapeInterval)
			recordMetrics(*scrapeURL)
		}
	}()

	http.Handle(*metricsPath, promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)

	logrus.Error(err)
}
