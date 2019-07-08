package main

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
    "flag"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/sirupsen/logrus"
)

var (
    addr = flag.String("listen", "localhost:9998", "The address to listen")
)

type Store struct {
	Success    int
	StatusCode int
	Bytes      int
	Labels     prometheus.Labels
}

func (s Store) Build() *prometheus.Registry {
	registry := prometheus.NewRegistry()

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

	registry.MustRegister(prometheus.NewGoCollector())

	return registry
}

func (s Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(s.Build(), promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func main() {
    flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<h1>hashcheck-exporter</h1><a href="/metrics">metrics</a>`)
	})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		target := q.Get("target")
		hash := strings.ToLower(q.Get("hash"))

        logrus.Debugf("request: %s %s", target, hash)

		if target == "" || hash == "" {
			http.Error(w, "error: not specified target or hash", http.StatusBadRequest)
			return
		}

		store := Store{Labels: prometheus.Labels{"target": target, "except": hash}}

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

				store.Labels["actual"] = actual
				if hash == actual {
					store.Success = 1
				}
			}
		}

		store.ServeHTTP(w, r)
	})

    logrus.Printf("listen on %s", *addr)
    logrus.Fatal(http.ListenAndServe(*addr, nil))
}
