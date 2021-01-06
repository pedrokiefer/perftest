package report

// count(count(galeb_http_requests_total{instance="$instance"}) by (virtualhost))
// count(count(galeb_http_requests_total{instance="$instance"}) by (exported_pool))
// count(count(galeb_http_requests_total{instance="$instance"}) by (rule))

// sum(rate(galeb_http_requests_total{instance="$instance"}[1m])) by (virtualhost)
// sum(rate(galeb_errors_total{instance="$instance"}[1m])) by (virtualhost, error)

// process_open_fds{instance="$instance"}

// memarea = label_values(jvm_memory_bytes_used{job="$job", instance="$instance"}, area)
// jvm_memory_bytes_used{area="$memarea",job="$job",instance="$instance"}
// jvm_memory_bytes_max{area="$memarea",job="$job",instance="$instance"}
// jvm_memory_bytes_used{area="$memarea",job="$job",instance="$instance"} / jvm_memory_bytes_max >= 0

// mempool = label_values(jvm_memory_pool_bytes_max{job="$job", instance="$instance"}, exported_pool)
// jvm_memory_pool_bytes_max{exported_pool="$mempool",job="$job",instance="$instance"}
// jvm_memory_pool_bytes_used{exported_pool="$mempool",job="$job",instance="$instance"}
// jvm_memory_pool_bytes_committed{exported_pool="$mempool",job="$job",instance="$instance"}

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/prometheus/prometheus/promql"
	chart "github.com/wcharczuk/go-chart/v2"

	"github.com/galeb/perftest/prom"
)

type Report struct {
	*prom.EmbeddedPrometheus
	reportDate time.Time
}

func New(e *prom.EmbeddedPrometheus) *Report {
	return &Report{
		EmbeddedPrometheus: e,
		reportDate:         time.Now(),
	}
}

func (r *Report) Generate() {
	memoryAreas := r.MemoryAreaLabels()
	for _, l := range memoryAreas {
		r.PlotMemoryArea(l)
	}
	_ = r.MemoryPoolsLabels()
}

func (r *Report) PlotMemoryArea(area string) {
	result := r.EmbeddedPrometheus.ExecuteRangeQuery(fmt.Sprintf(`jvm_memory_bytes_used{area="%s"}`, area), 1*time.Hour)
	if result.Err != nil {
		log.Fatalf("Could not execute query: err=%s", result.Err.Error())
	}

	used := chart.ContinuousSeries{
		Name: fmt.Sprintf("Used memory [%s]", area),
	}
	switch v := result.Value.(type) {
	case promql.Matrix:
		for _, item := range v {
			used.XValues = append(used.XValues, item.Points[0].V)
			used.YValues = append(used.XValues, item.Points[1].V)
		}
	}

	graph := chart.Chart{
		Series: []chart.Series{
			used,
		},
	}

	f, _ := os.Create(fmt.Sprintf("memory_area_%s.png", r.reportDate.Format("2006_01_02T15_04_05")))
	defer f.Close()
	graph.Render(chart.SVG, f)
}

func (r *Report) MemoryAreaLabels() []string {
	labels := []string{}
	result := r.EmbeddedPrometheus.ExecuteInstantQuery("jvm_memory_bytes_used")
	if result.Err != nil {
		log.Fatalf("Could not execute query: err=%s", result.Err.Error())
	}

	switch v := result.Value.(type) {
	case promql.Vector:
		for _, item := range v {
			a := item.Metric.Get("area")
			if !valueInSlice(a, labels) {
				labels = append(labels, a)
			}
		}
	}
	return labels
}

func (r *Report) MemoryPoolsLabels() []string {
	labels := []string{}
	result := r.EmbeddedPrometheus.ExecuteInstantQuery("jvm_memory_pool_bytes_max")
	if result.Err != nil {
		log.Fatalf("Could not execute query: err=%s", result.Err.Error())
	}

	switch v := result.Value.(type) {
	case promql.Vector:
		for _, item := range v {
			p := item.Metric.Get("pool")
			if !valueInSlice(p, labels) {
				labels = append(labels, p)
			}
		}
	}
	return labels
}

func valueInSlice(v string, s []string) bool {
	for _, i := range s {
		if i == v {
			return true
		}
	}
	return false
}
