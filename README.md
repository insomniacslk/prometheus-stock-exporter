# prometheus-stock-exporter

This is a stock price exporter for Prometheus. It uses the [FinnHub.io](https://finnhub.io)
API to get stock prices.

## Metrics

The exported metrics are named like `stock_<metric>`, where `<metric>` is what
you define in the configuration file as explained below.

## Configuration file

Create a configuration file similar to the following:

```
{
    "symbols": ["FB", "AMZN", "GOOG", "NFLX", "AAPL"],
    "frequency": "5m",
    "finnhub_api_key": "your-api-key"
}
```

Where:
* `symbols` is the list of symbols to track
* `frequency` is how frequently the stock price is retrieved for each symbol
* `finnhub_api_key` is self-explanatory


## Run it

```
go build
./prometheus-stock-exporter -c /path/to/your-config.json
```

## Grafana

See dashboard at
[dashboard.json](https://github.com/insomniacslk/prometheus-stock-exporter/blob/main/dashboard.json)
