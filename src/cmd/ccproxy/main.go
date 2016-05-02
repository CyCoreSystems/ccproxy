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

	"golang.org/x/net/context"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make sure /tmp exists
	os.Mkdir("/tmp", 0777)

	// Start the services manager
	err := services.Go(ctx)
	if err != nil {
		fmt.Println("Failed to start service manager:", err.Error())
		return
	}

	// Start the dns manager
	err = dns.Go(ctx)
	if err != nil {
		fmt.Println("Failed to start dns manager:", err.Error())
		return
	}

	// Wait for a stop signal
	// Wait for OS to signal stop
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	select {
	case <-c:
		fmt.Println("Got signal; stopping")
	case <-ctx.Done():
	}

	return
}
