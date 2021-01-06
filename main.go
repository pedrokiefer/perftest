package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/galeb/perftest/prom"
	"github.com/galeb/perftest/report"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	var endpoint string
	var metrics string
	var testDuration int
	var vhosts int
	var parallel int
	flag.StringVar(&endpoint, "endpoint", "", "Endpoint to Test")
	flag.StringVar(&metrics, "metrics", "", "Endpoint to Test")
	flag.IntVar(&testDuration, "duration", 5, "Test duration (in minutes)")
	flag.IntVar(&vhosts, "vhosts", 5, "Number of vhosts to test")
	flag.IntVar(&parallel, "parallel", 5, "Number of concurrent requests")
	flag.Parse()

	// hosts := genVirtuaHosts(vhosts)

	p := prom.New(metrics, time.Duration(10)*time.Second)
	p.Start()

	_, cancel := context.WithCancel(context.Background())
	// for i := 0; i < parallel; i++ {
	// 	log.Printf("%d parallel started", i)
	// 	go func() {
	// 		for {
	// 			select {
	// 			case <-time.After(10 * time.Second):
	// 				picked := rand.Intn(vhosts)
	// 				log.Printf("picked: [%s]", hosts[picked])
	// 				client.DoFireAndForgetHTTPReq(endpoint, hosts[picked], 10*time.Second, nil, nil)
	// 			case <-ctx.Done():
	// 				return
	// 			}
	// 		}
	// 	}()
	// }

	// // __ping__
	// go func() {
	// 	for {
	// 		log.Print("picked: [__ping__]")
	// 		select {
	// 		case <-time.After(30 * time.Second):
	// 			client.DoFireAndForgetHTTPReq(endpoint, "__ping__", 10*time.Second, nil, nil)
	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	// // __info__
	// go func() {
	// 	for {
	// 		log.Print("picked: [__info__]")
	// 		select {
	// 		case <-time.After(30 * time.Second):
	// 			client.DoFireAndForgetHTTPReq(endpoint, "__info__", 10*time.Second, nil, nil)
	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	select {
	case <-time.After(30 * time.Second):
		cancel()
	}

	r := report.New(p)
	r.Generate()

	p.Stop()
}

func genVirtuaHosts(n int) []string {
	r := []string{}
	for i := 0; i < n; i++ {
		r = append(r, fmt.Sprintf("galeb-test-%d", i))
	}
	return r
}
