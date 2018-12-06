package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Financial-Times/log-collector/filter"
	"github.com/Financial-Times/log-collector/forwarder"
)

var logsReader io.Reader

func init() {
	flag.StringVar(&forwarder.Env, "env", "dummy", "Environment tag value")
	flag.IntVar(&forwarder.Workers, "workers", 8, "Number of concurrent Workers")
	flag.IntVar(&forwarder.ChanBuffer, "buffer", 256, "Channel buffer size")
	flag.IntVar(&forwarder.Batchsize, "batchsize", 10, "Number of messages to group (before writing to S3 and delivering to Splunk HEC)")
	flag.IntVar(&forwarder.Batchtimer, "batchtimer", 5, "Expiry in seconds after which delivering events to S3")
	flag.StringVar(&forwarder.Bucket, "bucketName", "", "S3 Bucket where all the log events will be forwarded and stored")
	flag.StringVar(&forwarder.AwsRegion, "awsRegion", "", "AWS region for S3")
	flag.StringVar(&filter.DNSAddress, "dnsAddress", "", "The DNS entry of the full cluster, in case this env is regional. Example upp-prod-delivery.ft.com")
}

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}
	filter.Env = forwarder.Env
	validateConfig()

	forwarderIn, logFilterOut := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)

	go launchForwarder(forwarderIn, &wg)
	go watchTerminationSignals(logFilterOut)

	if logsReader == nil {
		logsReader = os.Stdin
	}

	filter.Filter(logsReader, logFilterOut)
	log.Println("Log filter completed")

	// closing the writer will finish the forwarder
	closeWriter(logFilterOut)

	wg.Wait() //wait for forwarder to complete.
}

func validateConfig() {
	if len(forwarder.Bucket) == 0 { //Check whether -Bucket parameter value was provided
		flag.Usage()
		os.Exit(1) //If not fail visibly as we are unable to send logs to S3
	}
}

func launchForwarder(forwarderIn io.Reader, wg *sync.WaitGroup) {
	forwarder.Forward(forwarderIn)
	wg.Done()
}

func watchTerminationSignals(logFilterOut io.Closer) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("Received shutdown signal: exiting gracefully")
	// closing the writer will finish the forwarder
	closeWriter(logFilterOut)
}

func closeWriter(logFilterOut io.Closer) {
	if err := logFilterOut.Close(); err != nil {
		log.Fatal(err, "Could not close the log filter writer")
	}
}
