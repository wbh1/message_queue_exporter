package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"

	"bytes"
	"os/exec"
	"strings"
)

var (
	// the label names we use for each distinct queue
	queueLabelNames = []string{"message_queue_id", "owner"}
)

// NewQueue creates a new Gauge vector with label names specified above and returns it
func NewQueue() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "nagios_message_queue",
			Name:      "length",
			Help:      "length of the kernel message queue",
		},
		queueLabelNames,
	)
}

type msqExporter struct {
	queues map[int]*prometheus.GaugeVec
}

func newMsgExporter() (*msqExporter, error) {
	messagequeues, err := getQueues()
	if err != nil {
		return nil, err
	}

	return &msqExporter{
		// queues is a map of kernel message queues discovered by getQueues()
		queues: messagequeues,
	}, nil
}

// Exporter is a basic type that shows if the exporter is up
type Exporter struct {
	up prometheus.Gauge
}

// NewExporter returns an initialized Exporter
func NewExporter() (*Exporter, error) {
	log.Infoln("Starting message queue exporter")

	return &Exporter{
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "nagios_message_queue",
			Name:      "up",
			Help:      "was the last message queue scrape successful",
		}),
	}, nil
}

// Describe describes all the metrics ever exported by the message queue exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up.Desc()
}

func (msq *msqExporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range msq.queues {
		m.Describe(ch)
	}
}

// Collect fetches the stats from ipcs -q and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	msqExporter, err := newMsgExporter()

	if err != nil {
		log.Errorf("Cannot get message queue(s): %v", err)
		e.up.Set(0)
	}

	if len(msqExporter.queues) != 0 {
		prometheus.Unregister(msqExporter)
		prometheus.MustRegister(msqExporter)
	}

	e.up.Set(1)

	ch <- e.up
}

func (msq *msqExporter) Collect(ch chan<- prometheus.Metric) {
	msq.resetMetrics()

	msq.scrape()
	msq.collectMetrics(ch)

}

// resetMetrics does what it says
func (msq *msqExporter) resetMetrics() {
	for _, m := range msq.queues {
		m.Reset()
	}
}

// Collect metrics for each queue
func (msq *msqExporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range msq.queues {
		m.Collect(metrics)
	}
}

// Scrape actually sets the metric values
func (msq *msqExporter) scrape() {

	cmd := exec.Command("ipcs", "-q")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Errorf("Cannot get message queue(s): %v", err)
		return
	}

	results := strings.Split(out.String(), "\n")[3:]

	for i := 0; i < len(results); i++ {
		a := strings.Fields(results[i])
		if len(a) == 6 {
			msgqID := string(a[1])
			owner := string(a[2])
			length, err := strconv.ParseFloat(string(a[5]), 64)
			if err != nil {
				log.Errorf("Cannot parse queue length: %v", err)
				continue
			}
			// For each queue, set its value with corresponding labels.
			msq.exportQueue(i, msgqID, owner, length)
		}
	}
}

func (msq *msqExporter) exportQueue(index int, msqgID string, owner string, length float64) {
	// Only set the metric if there is a metric in the queues map for it
	if len(msq.queues) >= index+1 {
		msq.queues[index].WithLabelValues(msqgID, owner).Set(length)
	} else {
		log.Errorf("Attempt to set queue value failed. Index outside of range")
	}
}

func getQueues() (map[int]*prometheus.GaugeVec, error) {

	queues := map[int]*prometheus.GaugeVec{}

	cmd := exec.Command("ipcs", "-q")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot get message queue(s): %v", err)
	}

	results := strings.Split(out.String(), "\n")[3:]

	for i := 0; i < len(results); i++ {
		a := strings.Fields(results[i])
		if len(a) == 6 {
			queues[i] = NewQueue()
		}
	}

	return queues, nil

}

func main() {

	var (
		listenaddress = flag.String("listen-address", ":8080", "Address on which to expose metrics.")
		metricspath   = flag.String("telemetry-path", "/metrics", "Path under which to expose metrics.")
	)

	flag.Parse()

	exporter, err := NewExporter()
	if err != nil {
		log.Fatal("Unable to create the exporter: %#v", err)
	}

	prometheus.MustRegister(exporter)

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle(*metricspath, promhttp.Handler())
	log.Infoln("Listening on port %v... \n", *listenaddress)
	log.Fatal(http.ListenAndServe(*listenaddress, nil))
}
