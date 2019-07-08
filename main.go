package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	addr   = flag.String("listen", "0.0.0.0:9998", "The address to listen")
	config = flag.String("config", "./hashcheck.yml", "Path to configuration file")
)

type Target struct {
	Url  string
	Hash string

	success       prometheus.Gauge
	changes       prometheus.Counter
	statusCode    prometheus.Gauge
	responseBytes prometheus.Gauge
	duration      prometheus.Gauge
	correct       prometheus.Gauge
	lastHash      string
}

func (t *Target) Init() {
	labels := prometheus.Labels{"target": t.Url}

	t.success = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_success",
		Help:        "1 if succeed to checking hash else 0",
		ConstLabels: labels,
	})
	t.changes = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "hashcheck_change_count",
		Help:        "number of page changes",
		ConstLabels: labels,
	})
	t.statusCode = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_status_code",
		Help:        "status code of HTTP response",
		ConstLabels: labels,
	})
	t.responseBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_response_bytes",
		Help:        "bytes of HTTP response",
		ConstLabels: labels,
	})
	t.duration = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hashcheck_duration_seconds",
		Help:        "taken time to HTTP fetch",
		ConstLabels: labels,
	})
	if t.Hash != "" {
		t.correct = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "hashcheck_correct",
			Help:        "1 if hash is correct else 0",
			ConstLabels: labels,
		})
	}
}

func (t *Target) Describe(ch chan<- *prometheus.Desc) {
	t.success.Describe(ch)
	t.changes.Describe(ch)
	t.statusCode.Describe(ch)
	t.responseBytes.Describe(ch)
	t.duration.Describe(ch)
	if t.Hash != "" {
		t.correct.Describe(ch)
	}
}

func (t *Target) Collect(ch chan<- prometheus.Metric) {
	t.success.Collect(ch)
	t.changes.Collect(ch)
	t.statusCode.Collect(ch)
	t.responseBytes.Collect(ch)
	t.duration.Collect(ch)
	if t.Hash != "" {
		t.correct.Collect(ch)
	}
}

func (t *Target) Probe() {
	t.success.Set(0)
	t.statusCode.Set(0)
	t.responseBytes.Set(0)
	if t.Hash != "" {
		t.correct.Set(0)
	}

	stime := time.Now()
	resp, err := http.Get(t.Url)
	etime := time.Now()

	t.duration.Set(etime.Sub(stime).Seconds())

	if err != nil {
		logrus.Errorf("failed to fetch [%s]: %s", t.Url, err)
		return
	}
	t.statusCode.Set(float64(resp.StatusCode))

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("failed to read body of [%s]: %s", t.Url, err)
		return
	}

	t.responseBytes.Set(float64(len(data)))
	t.success.Set(1)

	actual := fmt.Sprintf("%x", sha256.Sum256(data))

	if actual != t.lastHash {
		t.changes.Inc()
		t.lastHash = actual
	}

	if t.Hash != "" {
		if actual != t.Hash {
			logrus.Errorf("[%s]: hash is incorrect: excepted=%s actual=%s", t.Url, t.Hash, actual)
		} else {
			t.correct.Set(1)
		}
	}
}

type Watcher struct {
	Workers int
	Targets []*Target
}

func (w *Watcher) Init() {
	if w.Workers <= 0 {
		w.Workers = 3
	}

	for _, t := range w.Targets {
		t.Init()
	}
}

func (w *Watcher) Describe(ch chan<- *prometheus.Desc) {
	for _, t := range w.Targets {
		t.Describe(ch)
	}
}

func (w *Watcher) Collect(ch chan<- prometheus.Metric) {
	for _, t := range w.Targets {
		t.Collect(ch)
	}
}

func (w *Watcher) Probe() {
	wg := sync.WaitGroup{}
	ch := make(chan *Target, w.Workers)

	for i := 0; i < w.Workers; i++ {
		wg.Add(1)

		go (func() {
			defer wg.Done()

			for t := range ch {
				t.Probe()
			}
		})()
	}
	for _, t := range w.Targets {
		ch <- t
	}
	close(ch)
	wg.Wait()
}

func (w *Watcher) Build() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	registry.MustRegister(w)
	registry.MustRegister(prometheus.NewGoCollector())

	return registry
}

func (wa *Watcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wa.Probe()
	promhttp.HandlerFor(wa.Build(), promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func main() {
	flag.Parse()

	watcher := &Watcher{}
	fp, err := os.Open(*config)
	if err != nil {
		logrus.Fatalf("can't open configuration file: %s", *config)
	}
	yaml.NewDecoder(fp).Decode(watcher)
	watcher.Init()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<h1>hashcheck-exporter</h1><a href="/metrics">metrics</a>`)
	})
	http.Handle("/metrics", watcher)

	logrus.Printf("listen on %s", *addr)
	logrus.Fatal(http.ListenAndServe(*addr, nil))
}
