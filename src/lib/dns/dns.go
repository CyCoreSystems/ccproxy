package dns

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aetrion/dnsimple-go/dnsimple"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

var domain = "cycore.io"
var proxyName = "proxy"

const dnsNamespace = "/cycore/proxy/dns"

var dns *dnsimple.Client
var etcd client.Client

var (
	instanceID string
	proxyIPv4  string
	proxyIPv6  string

	dnsimpleToken string
	dnsimpleID    string // Account struct from login

	recordID4 int // DNS record ID for IPv4 entry
	recordID6 int // DNS record ID for IPv6 entry
)

// Go starts the dns manager
func Go(ctx context.Context) (err error) {
	instanceID = os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		return fmt.Errorf("INSTANCE_ID not defined")
	}

	proxyIPv4 = os.Getenv("COREOS_PUBLIC_IPV4")
	if proxyIPv4 == "" {
		return fmt.Errorf("COREOS_PUBLIC_IPV4 not defined")
	}

	proxyIPv6 = os.Getenv("COREOS_PUBLIC_IPV6")
	if proxyIPv6 == "" {
		return fmt.Errorf("COREOS_PUBLIC_IPV6 not defined")
	}

	dnsimpleToken = os.Getenv("DNSIMPLE_TOKEN")
	if dnsimpleToken == "" {
		return fmt.Errorf("DNSIMPLE_TOKEN not defined")
	}

	// Create a new dnsimple client
	dns = dnsimple.NewClient(dnsimple.NewOauthTokenCredentials(dnsimpleToken))

	// Store the account ID
	/*
		whoamiResponse, err := dns.Identity.Whoami()
		if err != nil {
			return fmt.Errorf("ERROR: could not fetch DNSimple ID: %s", err.Error())
		}
		dnsimpleID = strconv.Itoa(whoamiResponse.Data.Account.ID)
	*/

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
	err = Update(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Update sets the dns record
func Update(ctx context.Context) error {
	var err error
	var id int

	k := client.NewKeysAPI(etcd)

	// Update IPv4 address
	if recordID4 == 0 {
		id, err = create("A", proxyIPv4)
		if err != nil {
			return err
		}

		// Set the record ID in etcd
		_, err = k.Set(ctx, dnsNamespace+"/ipv4/"+instanceID, strconv.Itoa(id), nil)
		if err != nil {
			return err
		}

	} else {
		err = update(recordID4, proxyIPv4)
		if err != nil {
			fmt.Println("Failed to update DNS A record:", recordID4, proxyIPv4, err)
			return err
		}
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
		_, err = k.Set(ctx, dnsNamespace+"/ipv6/"+instanceID, strconv.Itoa(id), nil)
		if err != nil {
			return err
		}
	} else {
		err = update(recordID6, proxyIPv6)
		if err != nil {
			fmt.Println("Failed to update DNS AAAA record:", recordID6, proxyIPv6, err)
			return err
		}
	}
	return nil
}

func create(recordType string, recordValue string) (int, error) {
	r := dnsimple.ZoneRecord{
		Name:    proxyName,
		Type:    recordType,
		Content: recordValue,
		TTL:     60,
	}
	fmt.Printf("Creating DNS Record: %+v\n", r)

	resp, err := dns.Zones.CreateRecord("_", domain, r)

	return resp.Data.ID, err
}

func update(id int, recordValue string) (err error) {
	// First, check the existing value to see if we need to change it
	resp, err := dns.Zones.GetRecord("_", domain, id)
	if err != nil {
		return
	}
	if resp.Data.Content == recordValue {
		fmt.Printf("No update needed\n")
		return
	}

	_, err = dns.Zones.UpdateRecord("_", domain, id, dnsimple.ZoneRecord{
		Content: recordValue,
	})
	return
}
