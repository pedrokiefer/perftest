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
	baseDir    string
}

func New(e *prom.EmbeddedPrometheus) *Report {
	r := &Report{
		EmbeddedPrometheus: e,
		reportDate:         time.Now(),
	}
	r.baseDir = fmt.Sprintf("report_%s", r.reportDate.Format("2006_01_02T15_04_05"))
	os.Mkdir(r.baseDir, os.ModePerm)
	return r
}

func (r *Report) Generate() {
	memoryAreas := r.MemoryAreaLabels()
	for _, l := range memoryAreas {
		r.PlotMemoryArea(l)
	}
	_ = r.MemoryPoolsLabels()
}

func (r *Report) GenerateSeries(name, query string, d time.Duration) *chart.TimeSeries {
	result := r.EmbeddedPrometheus.ExecuteRangeQuery(query, d)
	if result.Err != nil {
		log.Fatalf("Could not execute query: err=%s", result.Err.Error())
	}

	ts := chart.TimeSeries{
		Name: name,
	}
	switch v := result.Value.(type) {
	case promql.Matrix:
		log.Printf("==== Matrix ===\n%#v\n", v)
		for _, serie := range v {
			for _, s := range serie.Points {
				ts.XValues = append(ts.XValues, time.Unix(s.T, 0))
				ts.YValues = append(ts.YValues, s.V)
			}
		}
	}

	return &ts
}

func (r *Report) PlotMemoryArea(area string) {
	used := r.GenerateSeries(
		fmt.Sprintf("Used memory [%s]", area),
		fmt.Sprintf(`jvm_memory_bytes_used{area="%s"}`, area),
		1*time.Hour)

	max := r.GenerateSeries(
		fmt.Sprintf("Max memory [%s]", area),
		fmt.Sprintf(`jvm_memory_bytes_max{area="%s"}`, area),
		1*time.Hour)

	usage := r.GenerateSeries(
		fmt.Sprintf("Usage memory [%s]", area),
		fmt.Sprintf(`jvm_memory_bytes_used{area="%s"} / jvm_memory_bytes_max >= 0`, area),
		1*time.Hour)

	graph := chart.Chart{
		Series: []chart.Series{
			used,
			max,
			usage,
		},
	}

	f, _ := os.Create(r.genFilename(fmt.Sprintf("memory_area_%s", area)))
	defer f.Close()
	graph.Render(chart.SVG, f)
}

func (r *Report) genFilename(plot string) string {
	return fmt.Sprintf("%s/%s.svg", r.baseDir, plot)
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
