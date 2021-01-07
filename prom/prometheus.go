// Package prom Embedded Prometheus based on https://github.com/wpjunior/epimetheus
package prom

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/textparse"
	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/teststorage"
)

type EmbeddedPrometheus struct {
	ScrapeURL string
	Interval  time.Duration
	Ticker    *time.Ticker

	Expression string

	Storage *teststorage.TestStorage
	Engine  *promql.Engine

	stop chan bool
	req  *http.Request
}

func New(url string, interval time.Duration) *EmbeddedPrometheus {
	ep := &EmbeddedPrometheus{
		ScrapeURL: url,
		Interval:  interval,
		Ticker:    time.NewTicker(interval),
	}

	ep.Storage = teststorage.New(&log.Logger{})
	ep.Engine = promql.NewEngine(promql.EngineOpts{
		MaxSamples:    50000000,
		LookbackDelta: time.Minute * 5,
		Timeout:       time.Second * 10,
	})

	ep.stop = make(chan bool)

	return ep
}

func (ep *EmbeddedPrometheus) Close() {
	ep.stop <- true
	ep.Storage.Close()
}

func (ep *EmbeddedPrometheus) Start() {
	ep.Ticker.Reset(ep.Interval)
	go func() {
		for {
			select {
			case <-ep.stop:
				ep.Ticker.Stop()
				return
			case <-ep.Ticker.C:
				ep.Scrape(context.Background())
			}
		}
	}()
}

func (ep *EmbeddedPrometheus) Stop() {
	ep.stop <- true
}

func (ep *EmbeddedPrometheus) Scrape(ctx context.Context) {
	buf := bytes.NewBuffer([]byte{})
	contentType, err := ep.readerForURL(ctx, buf)
	if err != nil {
		log.Fatal(err.Error())
	}

	b := buf.Bytes()
	err = ep.Append(b, contentType)
	if err != nil {
		log.Fatalf("Could not ingest prometheus metrics: err=%s", err.Error())
	}

	if ep.Expression != "" {
		result := ep.ExecuteInstantQuery(ep.Expression)
		if result.Err != nil {
			log.Fatalf("Could not execute query: err=%s", result.Err.Error())
		}

		switch v := result.Value.(type) {
		case promql.Vector:
			for _, item := range v {
				fmt.Println(item.Metric, item.Point.V)
			}
		}
	}

}

func (ep *EmbeddedPrometheus) ExecuteInstantQuery(expr string) *promql.Result {
	query, err := ep.Engine.NewInstantQuery(ep.Storage, expr, time.Now())
	if err != nil {
		log.Fatalf("Could not create query: err=%s", err.Error())
	}
	return query.Exec(context.Background())
}

func (ep *EmbeddedPrometheus) ExecuteRangeQuery(expr string, r time.Duration) *promql.Result {
	end := time.Now()
	begin := end.Add(-r)
	query, err := ep.Engine.NewRangeQuery(
		ep.Storage,
		expr,
		begin, end,
		15*time.Second)

	if err != nil {
		log.Fatalf("Could not create query: err=%s", err.Error())
	}
	return query.Exec(context.Background())
}

func (ep *EmbeddedPrometheus) readerForURL(ctx context.Context, w io.Writer) (string, error) {
	if ep.req == nil {
		req, err := http.NewRequest(http.MethodGet, ep.ScrapeURL, nil)
		if err != nil {
			return "", err
		}
		ep.req = req
	}
	resp, err := http.DefaultClient.Do(ep.req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("Could not get URL: %s, err=%s", ep.ScrapeURL, err.Error())
	}

	if resp.StatusCode >= http.StatusBadRequest {
		resp.Body.Close()
		return "", fmt.Errorf("Could not get URL: %s, statusCode=%d", ep.ScrapeURL, resp.StatusCode)
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return "", err
	}

	return resp.Header.Get("Content-Type"), nil
}

func (ep *EmbeddedPrometheus) Append(b []byte, contentType string) (err error) {
	appender := ep.Storage.Appender(context.Background())
	p := textparse.New(b, contentType)
	defTime := timestamp.FromTime(time.Now())

loop:
	for {
		var et textparse.Entry
		if et, err = p.Next(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		switch et {
		case textparse.EntryType:
			// ep.cache.setType(p.Type())
			continue
		case textparse.EntryHelp:
			// ep.cache.setHelp(p.Help())
			continue
		case textparse.EntryUnit:
			// ep.cache.setUnit(p.Unit())
			continue
		case textparse.EntryComment:
			continue
		default:
		}
		t := defTime
		met, tp, v := p.Series()
		if tp != nil {
			t = *tp
		}

		var lset labels.Labels
		p.Metric(&lset)
		// The label set may be set to nil to indicate dropping.
		if lset == nil {
			continue
		}

		// log.Printf("Metric: %#v V: %#v t: %#v", lset, v, t)

		if !lset.Has(labels.MetricName) {
			err = fmt.Errorf("missing metric name (%s label)", labels.MetricName)
			break loop
		}

		_, err = appender.Add(lset, t, v)
		if err != nil {
			if err != storage.ErrNotFound {
				err = fmt.Errorf("Unexpected error series %s err %v", string(met), err)
			}
			break loop
		}
	}
	if err == nil {
		appender.Commit()
	}
	return
}
