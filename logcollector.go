package main

import (
	"flag"
	"github.com/Financial-Times/log-collector/forwarder"
	"github.com/Financial-Times/log-collector/logfilter"
	"io"
	"os"
)

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	forwarderIn, logFilterOut := io.Pipe()
	go logfilter.LogFilter(os.Stdin, logFilterOut)
	go forwarder.Forward(forwarderIn)
}
