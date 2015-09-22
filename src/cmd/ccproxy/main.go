// ccproxy - coprocess for haproxy which manages its configuration
//   and reloads it, as necessary, based on changes in the configuration
//   of services in the cluster
package main

import (
	"fmt"
	"lib/dns"
	"lib/services"
	"os"
	"os/signal"
	"time"
)

func main() {
	stopChan := make(chan struct{})

	// Start the services manager
	err := services.Go(stopChan)
	if err != nil {
		fmt.Println("Failed to start service manager:", err.Error())
		return
	}

	// Start the dns manager
	err = dns.Go()
	if err != nil {
		fmt.Println("Failed to start dns manager:", err.Error())
		return
	}

	// Wait for a stop signal
	waitStop(stopChan)
	return
}

func waitStop(stopChan chan struct{}) {
	// Wait for OS to signal stop
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	<-c
	fmt.Println("Got signal; stopping")

	// Tell our children to stop gracefully
	close(stopChan)

	// Give them 1s to comply
	time.Sleep(1 * time.Second)

	return
}
