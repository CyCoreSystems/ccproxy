package services

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const MaximumAge = time.Duration(1 * time.Hour)

// A Backend represents a service provider within the cluster
type Backend struct {
	Name     string    // Name/Value of the backend
	LastSeen time.Time // Timestamp of last visibility
}

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
func parseBackend(node *client.Node) Backend {
	return Backend{
		Name:     node.Value,
		LastSeen: time.Now(),
	}
}

// Equals determines if two backends are equivalent
func (b *Backend) Equals(n *Backend) bool {
	return b.Name == n.Name
}

// Merge combines two lists of backends, taking
// the newer entries
func (b Backends) Merge(n Backends) *Backends {
	m := map[string]Backend{}

	// Add the old entries
	for _, i := range b {
		m[i.Name] = i
	}

	// Merge in the new entries
	for _, i := range n {
		if comp, ok := m[i.Name]; ok {
			if comp.LastSeen.After(i.LastSeen) {
				continue
			}
		}
		m[i.Name] = i
	}

	// Return the resulting list
	ret := Backends{}
	for _, i := range m {
		ret = append(ret, i)
	}
	return &ret
}

// Expire returns a list of backends whose ages
// are less than MaximumAge
func (b Backends) Expire() Backends {
	ret := Backends{}
	for _, i := range b {
		if time.Since(i.LastSeen) < MaximumAge {
			ret = append(ret, i)
		}
	}
	return ret
}

// Equals determines if two backend lists are equivalent
func (b Backends) Equals(n Backends) bool {
	// Test the length first
	if len(b) != len(n) {
		fmt.Println("Backends length differs", len(b), len(n))
		return false
	}

	// Test each backend
	var found bool
	for _, i := range b {
		found = false
		for _, j := range n {
			if i.Equals(&j) {
				found = true
			}
		}
		if !found {
			fmt.Println("Backend not found in list", i, n)
			return false
		}
	}
	return true
}
