package services

import (
	"fmt"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// A Backend represents a service provider within the cluster
type Backend string

// Backends represents a set of backends
type Backends []Backend

// backendsFor extracts the known backends for the
// given service name
func backendsFor(service string) (Backends, error) {
	// Get a keysAPI instance
	k := client.NewKeysAPI(etcd)

	// Get the service keys
	keyName := registratorNamespace + "/" + service
	resp, err := k.Get(context.Background(), keyName, &client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    false,
	})
	if err != nil {
		return nil, err
	}

	backends := Backends{}
	for _, b := range resp.Node.Nodes {
		backends = append(backends, parseBackend(b))
	}
	if len(backends) < 1 {
		return nil, fmt.Errorf("No backends found")
	}
	return backends, nil
}

// parseBackend constructs a backend from an registrator value
func parseBackend(node *client.Node) (b Backend) {
	return Backend(node.Value)
}

// Equals determines if two backends are equivalent
func (b Backend) Equals(n Backend) bool {
	return b == n
}

// Equals determines if two backend lists are equivalent
func (b Backends) Equals(n Backends) bool {
	// Test the length first
	if len(b) != len(n) {
		return false
	}

	// Test each backend
	var found bool
	for _, i := range b {
		found = false
		for _, j := range n {
			if i.Equals(j) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
