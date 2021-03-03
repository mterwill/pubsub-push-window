package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"golang.org/x/sync/semaphore"
)

const (
	statSuccess   = "success"
	statRateLimit = "rate_limit"
)

// Hardcode stable colors since map iteration is not ordered.
// From https://echarts.apache.org/en/option.html#color.
var colors = map[string]string{
	statSuccess:   "#2f4554",
	statRateLimit: "#c23531",
}

type Stats struct {
	mu    sync.Mutex
	title string
	start int64
	data  map[string]map[int64]int64 // data[label][time] = count
}

func NewStats(title string) *Stats {
	return &Stats{
		title: title,
		start: time.Now().Unix(),
		data:  make(map[string]map[int64]int64),
	}
}

func (s *Stats) Render(w io.Writer) error {
	end := time.Now().Unix() - 1

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithLegendOpts(opts.Legend{Show: true}),
		charts.WithTitleOpts(opts.Title{Title: s.title}))

	var keys []int64
	for i := s.start; i <= end; i++ {
		keys = append(keys, i)
	}
	line.SetXAxis(keys)

	s.mu.Lock()
	for name, series := range s.data {
		data := make([]opts.LineData, len(keys))
		for i, k := range keys {
			data[i] = opts.LineData{Value: series[k]}
		}

		line.AddSeries(name, data, charts.WithLineStyleOpts(opts.LineStyle{
			Color: colors[name],
		}))
	}
	s.mu.Unlock()

	return line.Render(w)
}

func (s *Stats) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.Render(w); err != nil {
		log.Print("error writing response: ", err)
	}
}

func (s *Stats) Increment(label string) {
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	data := s.data[label]
	if data == nil {
		data = make(map[int64]int64)
		s.data[label] = data
	}

	data[now]++
}

func main() {
	bindAddress := flag.String("bind-address", "127.0.0.1", "")
	port := flag.String("port", "8080", "")
	limit := flag.Int64("limit", 5, "concurrent request limit")
	sleep := flag.Int64("sleep", 500, "sleep duration ms")
	block := flag.Bool("block", false, "fail fast if unable to acquire semaphore or block until it can be acquired")
	flag.Parse()

	stats := NewStats(fmt.Sprintf("%d concurrent requests %dms sleep", *limit, *sleep))
	http.Handle("/stats", stats)

	sem := semaphore.NewWeighted(*limit)
	var semAcquire func(ctx context.Context) error
	if *block {
		semAcquire = func(ctx context.Context) error {
			return sem.Acquire(ctx, 1)
		}
	} else {
		semAcquire = func(ctx context.Context) error {
			if sem.TryAcquire(1) {
				return nil
			}
			return errors.New("failed to acquire semaphore")
		}
	}

	http.Handle("/pubsub", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := semAcquire(r.Context()); err != nil {
			stats.Increment(statRateLimit)
			http.Error(w, "concurrent request limit", http.StatusTooManyRequests)
			return
		}
		defer sem.Release(1)

		time.Sleep(time.Duration(*sleep) * time.Millisecond) // simulate some work
		stats.Increment(statSuccess)
	}))

	if err := http.ListenAndServe(net.JoinHostPort(*bindAddress, *port), http.DefaultServeMux); err != nil {
		log.Fatal("error starting server: ", err)
	}
}
