package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	
	"os/exec"
	"strings"
	"bytes"
)

var (
	messages = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "nagios_message_queue",
		Help: "Size of message queue for Nagios.",
	}, func() float64 {
	cmd := exec.Command("ipcs", "-q")
	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	result := strings.Split(out.String(),"\n")[3]

	if len(strings.Fields(result)) == 6 {
		msgCount, err := strconv.ParseFloat(strings.Fields(result)[5], 64)
		if err != nil {
			log.Fatal(err)
		}
		return msgCount
	} else {
		// msgCount = -1
		return float64(-1)
	}
	})
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(messages)
}


func main() {

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())
	print("Listening on port 8080... \n")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
