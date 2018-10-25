package main

import (
	"flag"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"

	"bytes"
	"os/exec"
	"strings"
)

const (
	namespace = "nagios"
)

var (
	msgQueueDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "message_queue", "length"),
		"length of the kernel message queue",
		[]string{"message_queue_id", "owner"},
		nil,
	)
)

type msgQueue struct {
	owner  string
	id     string
	length float64
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
	ch <- msgQueueDesc
	ch <- e.up.Desc()
}

// Collect fetches the stats from ipcs -q and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	msgQueues, err := getMsgQueues()

	if err != nil {
		log.Errorf("Cannot get message queue(s): %v", err)
		e.up.Set(0)
		ch <- e.up
		return
	}

	for i := 0; i < len(msgQueues); i++ {
		ch <- prometheus.MustNewConstMetric(
			msgQueueDesc,
			prometheus.GaugeValue,
			msgQueues[i].length,
			msgQueues[i].id,
			msgQueues[i].owner,
		)
	}

	e.up.Set(1)

	ch <- e.up
}

// getMsgQueues finds all the message queues and returns a slice containing them
func getMsgQueues() ([]msgQueue, error) {

	cmd := exec.Command("ipcs", "-q")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Errorf("Cannot get message queue(s): %v", err)
		return nil, err
	}

	results := strings.Split(out.String(), "\n")[3:]
	var queues []msgQueue

	// iterate through the results
	for i := 0; i < len(results); i++ {
		a := strings.Fields(results[i])
		if len(a) == 6 {
			msgqID := string(a[1])
			owner := string(a[2])
			length, err := strconv.ParseFloat(string(a[5]), 64)
			if err != nil {
				log.Errorf("Cannot parse queue length: %v", err)
				length = 0
				continue
			}

			// Append each queue
			queues = append(queues, msgQueue{owner, msgqID, length})
		}
	}

	return queues, nil
}

func main() {

	var (
		listenaddress = flag.String("listen-address", "8080", "Port on which to expose metrics.")
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
	log.Infoln("Listening on port", *listenaddress)
	log.Fatal(http.ListenAndServe(":"+*listenaddress, nil))
}
