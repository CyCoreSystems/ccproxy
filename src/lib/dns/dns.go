package dns

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	"github.com/weppos/go-dnsimple/dnsimple"
	"golang.org/x/net/context"
)

var domain = "cycore.io"
var proxyName = "proxy.cycore.io"

const dnsNamespace = "/cycore/proxy/dns"

var dns *dnsimple.DomainsService
var etcd client.Client

var (
	instanceID string
	proxyIPv4  string
	proxyIPv6  string

	dnsimpleEmail string
	dnsimpleAPI   string

	recordID4 int // DNS record ID for IPv4 entry
	recordID6 int // DNS record ID for IPv6 entry
)

// Go starts the dns manager
func Go() (err error) {
	instanceID = os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		return fmt.Errorf("INSTANCE_ID not defined")
	}

	proxyIPv4 = os.Getenv("COREOS_PUBLIC_IPV4")
	if proxyIPv4 == "" {
		return fmt.Errorf("COREOS_PUBLIC_IPV4 not defined")
	}

	proxyIPv6 = os.Getenv("COREOS_PUBLIC_IPV4")
	if proxyIPv6 == "" {
		return fmt.Errorf("COREOS_PUBLIC_IPV6 not defined")
	}

	dnsimpleEmail = os.Getenv("DNSIMPLE_EMAIL")
	if dnsimpleEmail == "" {
		return fmt.Errorf("DNSIMPLE_EMAIL not defined")
	}

	dnsimpleAPI = os.Getenv("DNSIMPLE_API")
	if dnsimpleAPI == "" {
		return fmt.Errorf("DNSIMPLE_API not defined")
	}

	// Create a new dnsimple client
	dns = dnsimple.NewClient(dnsimpleAPI, dnsimpleEmail).Domains

	// Create a new etcd client
	etcd, err = client.New(client.Config{
		Endpoints: strings.Split(os.Getenv("ETCD_ENDPOINTS"), ","),
		Transport: client.DefaultTransport,
	})
	if err != nil {
		return err
	}

	// See if we already have a recordid in etcd
	k := client.NewKeysAPI(etcd)

	// Get IPv4 recordid
	resp, err := k.Get(context.Background(), dnsNamespace+"/ipv4/"+instanceID, nil)
	if err != nil {
		if err.(client.Error).Code != client.ErrorCodeKeyNotFound {
			return err
		}
	} else {
		recordID4, err = strconv.Atoi(resp.Node.Value)
		if err != nil {
			fmt.Println("Failed to parse etcd recordID4", err.Error())
		}
	}

	// Get IPv6 recordid
	resp, err = k.Get(context.Background(), dnsNamespace+"/ipv6/"+instanceID, nil)
	if err != nil {
		if err.(client.Error).Code != client.ErrorCodeKeyNotFound {
			return err
		}
	} else {
		recordID6, err = strconv.Atoi(resp.Node.Value)
		if err != nil {
			fmt.Println("Failed to parse etcd recordID6", err.Error())
		}
	}

	// Update the DNS record with our IP
	err = Update()
	if err != nil {
		return err
	}

	return nil
}

// Update sets the dns record
func Update() (err error) {
	var id int

	k := client.NewKeysAPI(etcd)

	// Update IPv4 address
	if recordID4 == 0 {
		id, err = create("A", proxyIPv4)
		if err != nil {
			return err
		}

		// Set the record ID in etcd
		_, err = k.Set(context.Background(), dnsNamespace+"/ipv4/"+instanceID, strconv.Itoa(id), nil)
		if err != nil {
			return err
		}

	} else {
		err = update(recordID4, proxyIPv4)
	}
	if err != nil {
		return err
	}

	// Update IPv6 address
	if recordID6 == 0 {
		id, err = create("AAAA", proxyIPv6)
		if err != nil {
			return err
		}

		// Set the record ID in etcd
		_, err = k.Set(context.Background(), dnsNamespace+"/ipv6/"+instanceID, strconv.Itoa(id), nil)
		if err != nil {
			return err
		}
	} else {
		err = update(recordID6, proxyIPv6)
	}
	return err
}

func create(recordType string, recordValue string) (int, error) {
	record, _, err := dns.CreateRecord(domain, dnsimple.Record{
		Name:    proxyName,
		Type:    recordType,
		Content: recordValue,
	})

	return record.Id, err
}

func update(id int, recordValue string) (err error) {
	_, _, err = dns.UpdateRecord(domain, id, dnsimple.Record{
		Content: recordValue,
	})
	return err
}
