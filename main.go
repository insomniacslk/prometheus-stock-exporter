package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
	"github.com/insomniacslk/xjson"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	flagPath       = flag.String("p", "/metrics", "HTTP path where to expose metrics to")
	flagListen     = flag.String("l", ":9103", "Address to listen to")
	flagConfigFile = flag.String("c", "config.json", "Configuration file")
)

// Config is the configuration file type.
type Config struct {
	Symbols       []string       `json:"symbols"`
	Frequency     xjson.Duration `json:"frequency"`
	FinnhubAPIKey string         `json:"finnhub_api_key"`
}

// LoadConfig loads the configuration file into a Config type.
func LoadConfig(filepath string) (*Config, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON config: %w", err)
	}
	return &config, nil
}

// NewStocksCollector returns a new StocksCollector.
func NewStocksCollector(ctx context.Context, client *finnhub.DefaultApiService, symbols []string) *StocksCollector {
	return &StocksCollector{
		ctx:     ctx,
		client:  client,
		symbols: symbols,
	}
}

// StocksCollector is a custom collector for point-in-time metrics that can
// be used as Grafana annotations.
type StocksCollector struct {
	ctx     context.Context
	client  *finnhub.DefaultApiService
	symbols []string
}

// Describe implements prometheus.Collector.Describe for StocksCollector.
func (sc *StocksCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(sc, ch)
}

var (
	companyNewsDesc = prometheus.NewDesc(
		"stock_company_news",
		"Stocks - Company News",
		[]string{"symbol", "headline", "url", "id"},
		nil,
	)
	stockPriceDesc = prometheus.NewDesc(
		"stock_price",
		"Stocks - Symbol price",
		[]string{"symbol"},
		nil,
	)
)

// Collect implements prometheus.Collector.Collect for StocksCollector.
func (sc *StocksCollector) Collect(ch chan<- prometheus.Metric) {
	// update company news as timestamped metric, useful for Grafana annotations
	today := time.Now().Format("2006-01-02")
	from, to := today, today
	fmt.Printf("Fetching company news for %v from %s to %s\n", sc.symbols, from, to)
	for _, sym := range sc.symbols {
		// collect stock price
		fmt.Printf("Getting stock price for %s\n", sym)
		resPrice, _, err := sc.client.Quote(sc.ctx).Symbol(sym).Execute()
		if err != nil {
			log.Printf("Failed to get stock price for '%s': %v", sym, err)
			continue
		}
		if resPrice.C == nil {
			log.Printf("Warning: skipping %s that has current price set to `nil`", sym)
		} else {
			// update values
			ch <- prometheus.MustNewConstMetric(stockPriceDesc, prometheus.GaugeValue, float64(*resPrice.C), sym)
		}

		// collect company news
		resNews, _, err := sc.client.CompanyNews(sc.ctx).Symbol(sym).From(from).To(to).Execute()
		if err != nil {
			fmt.Printf("Failed to get company news for '%s': %v\n", sym, err)
			continue
		}
		fmt.Printf("Found %d company news for %s\n", len(resNews), sym)
		for _, news := range resNews {
			if news.Datetime == nil || news.Headline == nil || news.Id == nil || news.Url == nil {
				fmt.Printf("Skipping company news for %s: found nil fields where non-nil wanted: %+v\n", sym, news)
				continue
			}
			// FIXME collect this metric exactly once
			ch <- prometheus.NewMetricWithTimestamp(
				time.Unix(*news.Datetime, 0),
				prometheus.MustNewConstMetric(
					companyNewsDesc,
					prometheus.GaugeValue,
					1,
					sym,
					*news.Headline,
					*news.Url,
					fmt.Sprintf("%d", *news.Id),
				),
			)
		}
	}

}

func main() {
	flag.Parse()
	config, err := LoadConfig(*flagConfigFile)
	if err != nil {
		log.Fatalf("Failed to load configuration file '%s': %v", *flagConfigFile, err)
	}
	fmt.Printf("Symbols (%d): %s\n", len(config.Symbols), config.Symbols)

	if len(config.Symbols) == 0 {
		log.Fatalf("Must specify at least one symbol")
	}

	// open finnhub client
	cfg := finnhub.NewConfiguration()
	cfg.AddDefaultHeader("X-Finnhub-Token", config.FinnhubAPIKey)
	cl := finnhub.NewAPIClient(cfg).DefaultApi
	ctx := context.Background()

	// register collectors
	stocksCollector := NewStocksCollector(ctx, cl, config.Symbols)
	if err := prometheus.Register(stocksCollector); err != nil {
		log.Fatalf("Failed to register stocks collector: %v", err)
	}

	http.Handle(*flagPath, promhttp.Handler())
	log.Printf("Starting server on %s", *flagListen)
	log.Fatal(http.ListenAndServe(*flagListen, nil))
}
