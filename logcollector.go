package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Financial-Times/log-collector/forwarder"
	"github.com/Financial-Times/log-collector/logfilter"
)

var logsReader io.Reader

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	forwarderIn, logFilterOut := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)

	go launchForwarder(forwarderIn, &wg)
	go watchTerminationSignals(logFilterOut)

	if logsReader == nil {
		logsReader = os.Stdin
	}

	logfilter.Filter(logsReader, logFilterOut)
	log.Println("Log filter completed")

	// closing the writer will finish the forwarder
	closeWriter(logFilterOut)

	wg.Wait() //wait for forwarder to complete.
}

func launchForwarder(forwarderIn io.Reader, wg *sync.WaitGroup) {
	forwarder.Forward(forwarderIn)
	log.Println("Forwarder completed")
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
