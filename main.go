package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenAddress := flag.String("web.listen-address", ":9828", "Address on which to expose metrics.")
	upstream := flag.String("uml.base-url", "https://www.uml.edu", "Base URL for the UMass Lowell APIs.")
	timeout := flag.Duration("uml.timeout", 10*time.Second, "Timeout for each upstream request.")
	flag.Parse()

	registry := prometheus.NewRegistry()
	registry.MustRegister(NewCollector(*upstream, *timeout))
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/-/healthy", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	log.Printf("listening on %s", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
