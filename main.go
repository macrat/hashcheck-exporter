package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	addr   = flag.String("listen", "localhost:9998", "The address to listen")
	config = flag.String("config", "./hashcheck.yml", "Path to configuration file")
)

type Store struct {
	Success    int
	StatusCode int
	Bytes      int
	Labels     prometheus.Labels
}

func (s Store) RegisterTo(registry *prometheus.Registry) {
	success := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_success",
		Help:        "1 if hash is correct else 0",
		ConstLabels: s.Labels,
	})
	success.Set(float64(s.Success))
	registry.MustRegister(success)

	statusCode := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_status_code",
		Help:        "status code of HTTP response",
		ConstLabels: s.Labels,
	})
	statusCode.Set(float64(s.StatusCode))
	registry.MustRegister(statusCode)

	bytes := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_bytes",
		Help:        "bytes of HTTP response",
		ConstLabels: s.Labels,
	})
	bytes.Set(float64(s.Bytes))
	registry.MustRegister(bytes)
}

func (s Store) Build() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	s.RegisterTo(registry)
	registry.MustRegister(prometheus.NewGoCollector())

	return registry
}

func (s Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(s.Build(), promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

type Config struct {
	Targets map[string]string `yaml:targets`
}

type StoreArray []Store

func (sa StoreArray) Build() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	for _, s := range sa {
		s.RegisterTo(registry)
	}

	registry.MustRegister(prometheus.NewGoCollector())

	return registry
}

func (sa StoreArray) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(sa.Build(), promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func main() {
	flag.Parse()

	var c Config
	fp, err := os.Open(*config)
	if err != nil {
		logrus.Fatalf("can't open configuration file: %s", *config)
	}
	yaml.NewDecoder(fp).Decode(&c)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<h1>hashcheck-exporter</h1><a href="/metrics">metrics</a>`)
	})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		stores := StoreArray{}

		for target, hash := range c.Targets {
			store := Store{Labels: prometheus.Labels{"target": target, "hash": hash}}

			if resp, err := http.Get(target); err != nil {
				logrus.Errorf("failed to fetch [%s]: %s", target, err)
			} else {
				store.StatusCode = resp.StatusCode

				data, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					logrus.Errorf("failed to read body of [%s]: %s", target, err)
				} else {
					store.Bytes = len(data)

					actual := fmt.Sprintf("%x", sha256.Sum256(data))

					if hash != actual {
						logrus.Errorf("[%s]: hash is incorrect: excepted=%s actual=%s", target, hash, actual)
					} else {
						store.Success = 1
					}
				}
			}

			stores = append(stores, store)
		}

		stores.ServeHTTP(w, r)
	})

	logrus.Printf("listen on %s", *addr)
	logrus.Fatal(http.ListenAndServe(*addr, nil))
}
