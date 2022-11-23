package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"speedtest-exporter/internal/app_info"
	"speedtest-exporter/internal/bandwidth_observer"
	"speedtest-exporter/internal/exporter"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	app_name = "speedtest-exporter"
	version  = "x.x.x"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func newHealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "OK")
	})
}

func main() {
	debug := flag.Bool("debug", false, "sets log level to debug")
	gracefulShutdown := flag.Bool("graceful-shutdown", true, "allow in flight speed tests to finish before shutting down")
	gracefulShutdownTimeout := flag.Duration("graceful-shutdown-timeout", 10*time.Second, "graceful shutdown timeout")
	testTimeout := flag.Duration("test-timeout", 1*time.Minute, "timeout for speedtest runs")
	testInterval := flag.Duration("test-interval", 1*time.Hour, "interval between speedtest runs")
	goCollector := flag.Bool("gocollector", false, "enables go stats exporter")
	processCollector := flag.Bool("processcollector", false, "enables process stats exporter")
	savingMode := flag.Bool("saving-mode", false, "enables saving mode in speedtest-go to reduce bandwidth usage at the cost of accuracy")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	var srv http.Server

	idleConnsClosed := make(chan struct{})
	exporterCtx, exporterCancel := context.WithCancel(context.Background())
	defer exporterCancel()

	go func() {
		sigchan := make(chan os.Signal, 1)

		signal.Notify(sigchan, os.Interrupt)
		signal.Notify(sigchan, syscall.SIGTERM)
		sig := <-sigchan
		log.Info().
			Str("signal", sig.String()).
			Msg("Stopping in response to signal")
		ctx, cancel := context.WithTimeout(context.Background(), *gracefulShutdownTimeout)
		defer cancel()
		if !*gracefulShutdown {
			log.Info().Msg("Canceling all in flight speed tests")
			exporterCancel()
		}
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("Failed to gracefully close http server")
		}
		close(idleConnsClosed)
	}()

	log.Info().
		Str("app_name", app_name).
		Str("version", version).
		Msg("Exporter Started.")

	appFunc := app_info.GaugeFunc(app_info.Opts{
		Namespace: "speedtest_exporter",
		Name:      app_name,
		Version:   version,
	})
	bw := bandwidth_observer.New(http.DefaultTransport)
	ex := exporter.New(exporter.Opts{
		Ctx:          exporterCtx,
		TestTimeout:  *testTimeout,
		TestInterval: *testInterval,
		Doer:         &http.Client{Transport: bw},
		SavingMode:   *savingMode,
	})

	go func() {
		log.Debug().Msg("Starting Result Update Thread")
		ex.TestLoop()
	}()

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(appFunc, ex, bw)

	if *goCollector {
		reg.MustRegister(collectors.NewGoCollector())
	}
	if *processCollector {
		reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	router.Handle("/healthz", newHealthCheckHandler())
	srv.Addr = ":8080"
	srv.Handler = router
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start HTTP Server")
	}
	<-idleConnsClosed
}
