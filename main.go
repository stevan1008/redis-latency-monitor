package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	cache "devops.bi.com.gt/BISistemas/Bi-en-linea-App-CI/_git/bi-bel3-cache-go.git"
	configjson "devops.bi.com.gt/BISistemas/Bi-en-linea-App-CI/_git/bi-bel3-configuration-go.git"
	_ "devops.bi.com.gt/BISistemas/Bi-en-linea-App-CI/_git/bi-bel3-redis-go.git"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mu      sync.Mutex
	latency []float64

	latencyHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "redis_latency_ms",
		Help:    "Latencia de peticiones Redis (ms)",
		Buckets: prometheus.LinearBuckets(0, 1, 50),
	})
	requestCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "redis_requests_total",
		Help: "Número total de peticiones realizadas a Redis",
	})
)

func init() {
	prometheus.MustRegister(latencyHistogram)
	prometheus.MustRegister(requestCounter)
}

func main() {
	cacheClient := cache.NewCache()
	if cacheClient == nil {
		log.Fatal("No se pudo inicializar la instancia de caché (Redis)")
	}
	fmt.Println("Redis inicializado correctamente (modo Bi)")

	reqs := configjson.GetInt("monitor.requests")
	if reqs == 0 {
		reqs = 1000
	}
	delay := time.Duration(configjson.GetInt("monitor.intervalMs")) * time.Millisecond
	if delay == 0 {
		delay = 500 * time.Millisecond
	}
	port := fmt.Sprintf(":%d", configjson.GetInt("monitor.port"))
	if port == ":0" {
		port = ":8080"
	}

	// Un solo servidor HTTP
	mux := http.NewServeMux()

	// Endpoint de métricas Prometheus
	mux.Handle("/metrics", promhttp.Handler())

	// Endpoint del dashboard (HTML con go-echarts)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		line := charts.NewLine()
		line.SetGlobalOptions(
			charts.WithTitleOpts(opts.Title{
				Title:    "Redis Latency Monitor",
				Subtitle: "Latencia promedio (ms) en tiempo real",
			}),
			charts.WithYAxisOpts(opts.YAxis{Name: "ms"}),
			charts.WithXAxisOpts(opts.XAxis{Name: "Request #"}),
		)

		mu.Lock()
		data := make([]opts.LineData, len(latency))
		for i, v := range latency {
			data[i] = opts.LineData{Value: v}
		}
		mu.Unlock()

		line.SetXAxis(generateXAxis(len(data))).AddSeries("Latency", data)
		line.Render(w)
	})

	// Lanzar el servidor
	go func() {
		log.Printf("Servidor HTTP escuchando en %s (paths: / y /metrics)\n", port)
		log.Fatal(http.ListenAndServe(port, mux))
	}()

	// Loop de medición
	for i := 0; i < reqs; i++ {
		start := time.Now()
		_, err := cacheClient.Get("tribalConnection")
		elapsed := time.Since(start)
		ms := float64(elapsed.Microseconds()) / 1000.0

		if err != nil && err.Error() != "redis: nil" {
			log.Printf("Error en request #%d: %v", i, err)
			continue
		}

		mu.Lock()
		latency = append(latency, ms)
		mu.Unlock()

		latencyHistogram.Observe(ms)
		requestCounter.Inc()

		if (i+1)%100 == 0 {
			fmt.Printf("[%d/%d] Latencia actual: %.2f ms\n", i+1, reqs, ms)
		}

		time.Sleep(delay)
	}

	select {}
}

func generateXAxis(n int) []int {
	x := make([]int, n)
	for i := 0; i < n; i++ {
		x[i] = i
	}
	return x
}
