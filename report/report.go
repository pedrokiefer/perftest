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
	images := []string{
		r.PlotRequests(),
		r.PlotRequestsErrors(),
		r.PlotOpenFileDescriptors(),
	}

	memoryAreas := r.MemoryAreaLabels()
	log.Printf("Memory Areas: %#v", memoryAreas)
	for _, l := range memoryAreas {
		images = append(images, r.PlotMemoryArea(l))
	}

	memoryPools := r.MemoryPoolsLabels()
	log.Printf("Memory Pools: %#v", memoryPools)
	for _, l := range memoryPools {
		images = append(images, r.PlotMemoryPool(l))
	}
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
		for _, serie := range v {
			for _, s := range serie.Points {
				ts.XValues = append(ts.XValues, time.Unix(s.T/1000, 0))
				ts.YValues = append(ts.YValues, s.V)
			}
		}
	}

	return &ts
}

func (r *Report) GenerateMultiSeries(query string, d time.Duration) []chart.Series {
	result := r.EmbeddedPrometheus.ExecuteRangeQuery(query, d)
	if result.Err != nil {
		log.Fatalf("Could not execute query: err=%s", result.Err.Error())
	}
	series := []chart.Series{}
	switch v := result.Value.(type) {
	case promql.Matrix:
		for _, serie := range v {
			vhost := serie.Metric.Get("virtualhost")
			ts := chart.TimeSeries{
				Name: vhost,
			}
			for _, s := range serie.Points {
				ts.XValues = append(ts.XValues, time.Unix(s.T/1000, 0))
				ts.YValues = append(ts.YValues, s.V)
			}
			series = append(series, ts)
		}
	}

	return series
}

func ByteCountSI(v interface{}) string {
	b := int(v.(float64))
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func (r *Report) PlotRequests() string {
	requests := r.GenerateMultiSeries(
		"sum(rate(galeb_http_requests_total[1m])) by (virtualhost)",
		1*time.Hour)

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Bottom: 20,
			},
		},
		YAxis: chart.YAxis{
			Name: "Requests / minute",
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
			GridMajorStyle: chart.Style{
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: requests,
	}

	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph),
	}

	f, _ := os.Create(r.genFilename("requests"))
	defer f.Close()
	graph.Render(chart.PNG, f)

	return "requests"
}

func (r *Report) PlotRequestsErrors() string {
	requests := r.GenerateMultiSeries(
		"sum(rate(galeb_errors_total[1m])) by (virtualhost, error)",
		1*time.Hour)

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Bottom: 20,
			},
		},
		YAxis: chart.YAxis{
			Name: "Requests / minute",
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
			GridMajorStyle: chart.Style{
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: requests,
	}

	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph),
	}

	f, _ := os.Create(r.genFilename("requests_errors"))
	defer f.Close()
	graph.Render(chart.PNG, f)

	return "requests_errors"
}

func (r *Report) PlotOpenFileDescriptors() string {
	fds := r.GenerateSeries(
		"Open File Descriptors",
		"process_open_fds",
		1*time.Hour)

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Bottom: 20,
			},
		},
		YAxis: chart.YAxis{
			Name: "File Descriptors",
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
			GridMajorStyle: chart.Style{
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: []chart.Series{
			fds,
		},
	}

	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph),
	}

	f, _ := os.Create(r.genFilename("open_fds"))
	defer f.Close()
	graph.Render(chart.PNG, f)

	return "open_fds"
}

func (r *Report) PlotMemoryArea(area string) string {
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

	usage.Style = chart.Style{
		StrokeColor: chart.ColorRed,
		FillColor:   chart.ColorRed.WithAlpha(100),
	}
	usage.YAxis = chart.YAxisSecondary

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Bottom: 20,
			},
		},
		YAxis: chart.YAxis{
			Name:           "Memory",
			ValueFormatter: ByteCountSI,
		},
		YAxisSecondary: chart.YAxis{
			Name: "% usage",
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
			GridMajorStyle: chart.Style{
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: []chart.Series{
			used,
			max,
			usage,
		},
	}

	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph),
	}

	filename := fmt.Sprintf("memory_area_%s", area)
	f, _ := os.Create(r.genFilename(filename))
	defer f.Close()
	graph.Render(chart.PNG, f)

	return filename
}

func (r *Report) PlotMemoryPool(pool string) string {
	used := r.GenerateSeries(
		fmt.Sprintf("Used memory [%s]", pool),
		fmt.Sprintf(`jvm_memory_pool_bytes_used{pool="%s"}`, pool),
		1*time.Hour)

	max := r.GenerateSeries(
		fmt.Sprintf("Max memory [%s]", pool),
		fmt.Sprintf(`jvm_memory_pool_bytes_max{pool="%s"}`, pool),
		1*time.Hour)

	commited := r.GenerateSeries(
		fmt.Sprintf("Usage memory [%s]", pool),
		fmt.Sprintf(`jvm_memory_pool_bytes_committed{pool="%s"}`, pool),
		1*time.Hour)

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Bottom: 20,
			},
		},
		YAxis: chart.YAxis{
			Name:           "Memory",
			ValueFormatter: ByteCountSI,
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
			GridMajorStyle: chart.Style{
				StrokeColor: chart.ColorAlternateGray,
				StrokeWidth: 1.0,
			},
		},
		Series: []chart.Series{
			used,
			max,
			commited,
		},
	}

	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph),
	}
	filename := fmt.Sprintf("memory_pool_%s", pool)
	f, _ := os.Create(r.genFilename(filename))
	defer f.Close()
	graph.Render(chart.PNG, f)

	return filename
}

func (r *Report) genFilename(plot string) string {
	return fmt.Sprintf("%s/%s.png", r.baseDir, plot)
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
