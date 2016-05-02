package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Etcd namespace roots
const serviceNamespace = "/cycore/proxy/services"
const dnsNamespace = "/cycore/proxy/dns"
const registratorNamespace = "/srv"

// The instance id
var instanceID string

// services is the internal list of all Services
var services map[string]*Service

// etcd is the reference to the etcd client
var etcd client.Client

// Service represents a discrete service in the proxy
type Service struct {
	Name     string   // Service Name
	DNS      []string // DNS hostnames for service
	Cert     string   // Certificates (PEM) for service
	Backends Backends // List of active backends for the service
}

func init() {
	// Store the instanceId for other components to reference
	instanceID = os.Getenv("INSTANCE_ID")
}

// Go starts the service manager, which monitors etcd,
// writes configurations, and updates haproxy, as necessary
func Go(ctx context.Context) (err error) {
	// Connect to etcd
	etcd, err = client.New(client.Config{
		Endpoints: strings.Split(os.Getenv("ETCD_ENDPOINTS"), ","),
		Transport: client.DefaultTransport,
	})
	if err != nil {
		return err
	}

	// Build the service list
	services = make(map[string]*Service)

	// Update immediately on first run
	err = Update()
	if err != nil {
		return err
	}

	// Watch for changes
	go Watch(ctx)

	return nil
}

// Watch watches all relevent etcd keys and updates
// the services on change
func Watch(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Run service watcher
	go watchServices(ctx)
	go watchBackends(ctx)

	// Wait for stop signal
	<-ctx.Done()
}

// Update regenerates the list of services and, if there
// were changes, writes the new configuration and reloads
// haproxy
func Update() error {
	changed, err := Load()
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	// Always call update on the first round
	//fmt.Println("Writing new configuration")
	err = Write()
	if err != nil {
		fmt.Println("Failed to write configuration:", err.Error())
		return err
	}

	//fmt.Println("Restarting haproxy")
	err = Reload()
	if err != nil {
		fmt.Println("Warning- Failed to restart haproxy:", err.Error())
		// Don't treat this as an error
	}

	return nil
}

// Load reads all the services from etcd
func Load() (changed bool, err error) {
	// Get a keysAPI instance
	k := client.NewKeysAPI(etcd)

	// Get the service keys
	resp, err := k.Get(context.Background(), serviceNamespace, &client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    false,
	})
	if err != nil {
		return false, err
	}

	// Parse each service
	for _, i := range resp.Node.Nodes {
		s, err := ParseServiceNode(i)
		if err != nil {
			continue
		}

		// If there are no changes; don't update the reference
		if old, ok := services[s.Name]; ok {
			if old.Equals(s) {
				continue
			}
		}

		// Add/update the service
		services[s.Name] = s
		changed = true
	}

	return changed, nil
}

// ParseServiceNode reads in a service node and returns a *Service for it
func ParseServiceNode(n *client.Node) (*Service, error) {
	serviceName := lastKeyName(n.Key)
	if serviceName == "" {
		return nil, fmt.Errorf("Failed to determine serviceName of node")
	}

	dnsNames := dnsFromKeys(n.Nodes)
	if len(dnsNames) < 1 {
		return nil, fmt.Errorf("No DNS names found for service")
	}

	cert := certFromKeys(n.Nodes)

	backends, err := backendsFor(serviceName)
	if err != nil {
		return nil, err
	}

	return &Service{
		Name:     serviceName,
		DNS:      dnsNames,
		Backends: backends,
		Cert:     cert,
	}, nil
}

// Return the last element of the key path
func lastKeyName(key string) string {
	pieces := strings.Split(key, "/")
	return pieces[len(pieces)-1]
}

// serviceFromRegistratorKey pulls the service name from a registrator key
func serviceFromRegistratorKey(key string) string {
	s := strings.TrimPrefix(key, registratorNamespace+"/")
	if s == key {
		// Not a registrator namespace key
		return ""
	}

	return strings.Split(s, "/")[0]
}

// Extract the dns entries from the service nodes
func dnsFromKeys(keys client.Nodes) []string {
	names := []string{}
	for _, n := range keys {
		if lastKeyName(n.Key) == "dns" {
			for _, d := range n.Nodes {
				names = append(names, d.Value)
			}
		}
	}
	return names
}

// certFromKeys extracts the cert (if present) from the
// etcd service nodes
func certFromKeys(keys client.Nodes) (cert string) {
	for _, n := range keys {
		if lastKeyName(n.Key) == "cert" {
			cert = n.Value
		}
	}
	return cert
}

// Equals determines if two services are equivalent
func (s *Service) Equals(n *Service) bool {
	if s.Name != n.Name {
		fmt.Println("Names differ:", s.Name, n.Name)
		return false
	}

	if s.Cert != n.Cert {
		fmt.Println("Certs differ:", s.Cert, n.Cert)
		return false
	}

	if len(s.DNS) != len(n.DNS) {
		fmt.Println("Number of DNS differ:", len(s.DNS), len(n.DNS))
		return false
	}
	var equal bool
	for _, i := range s.DNS {
		equal = false
		for _, j := range n.DNS {
			if i == j {
				equal = true
			}
		}
		if !equal {
			fmt.Println("DNS entry not found in list:", i, n.DNS)
			return false
		}
	}

	// Merge and expire old backends with new
	n.Backends = s.Backends.Merge(n.Backends).Expire()

	// Check for differences in backends list
	if !s.Backends.Equals(n.Backends) {
		fmt.Println("Backends differ")
		return false
	}

	return true
}

// Watch the services etcd tree for changes.  It calls
// Update if it detects changes.
func watchServices(ctx context.Context) {
	k := client.NewKeysAPI(etcd)
	watcher := k.Watcher(serviceNamespace, &client.WatcherOptions{
		Recursive: true,
	})

	// Process changes
	for {
		resp, err := watcher.Next(ctx)
		if err != nil {
			fmt.Println("Error watching services", err.Error())
			continue
		}

		// Ignore read-only events
		if resp.Action == "get" {
			continue
		}

		fmt.Println("Update of service configuration detected")
		Update()
	}
}

// watchBackends monitors the registrator namespace for
// changes affecting services which are monitored.  It
// calls update if there are changes detected.
func watchBackends(ctx context.Context) {
	k := client.NewKeysAPI(etcd)
	watcher := k.Watcher(registratorNamespace, &client.WatcherOptions{
		Recursive: true,
	})

	// Process changes
	for {
		resp, err := watcher.Next(ctx)
		if err != nil {
			fmt.Println("Error watching backends", err.Error())
			continue
		}

		// Ignore read-only events
		if resp.Action == "get" {
			continue
		}

		// Find the service of the modified key
		serviceName := serviceFromRegistratorKey(resp.Node.Key)
		if serviceName == "" {
			continue
		}

		// Update only if this service is one we are watching
		for _, s := range services {
			if serviceName == s.Name {
				Update()
				break
			}
		}
	}
}
