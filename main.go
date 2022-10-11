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
	lowestPrice = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oil_lowest_price",
		Help: "The lowest price per gallon available in USD",
	})
)

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

	selection := doc.Find("table.paywithcash tr:nth-child(3) td:last-child")

	if selection.Length() == 0 {
		logrus.Error("failed to find row in response document")
		return
	}

	contents := selection.First().Text()
	price, err := strconv.ParseFloat(contents[1:], 32)

	if err != nil {
		logrus.Errorf("failed to convert text to price: %v", err)
		return
	}

	price = math.Round(price*100) / 100

	logrus.Infof("low price found: %v", price)

	lowestPrice.Set(price)
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

	go func() {
		for {
			recordMetrics(*scrapeURL)
			time.Sleep(*scrapeInterval)
		}
	}()

	http.Handle(*metricsPath, promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)

	logrus.Error(err)
}
