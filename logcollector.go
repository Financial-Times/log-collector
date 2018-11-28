package main

import "flag"

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

}
