package services

import (
	"io/ioutil"
	"os"
	"text/template"

	"github.com/coreos/go-systemd/dbus"
	"github.com/termie/go-shutil"
)

var proxyTemplate *template.Template

const haproxyConfig = "/data/haproxy.cfg"
const haproxyCertsDir = "/data/certs"

var proxyIPv4 string
var proxyIPv6 string

type HAProxyConfig struct {
	IPv4     string
	IPv6     string
	Services map[string]*Service
}

func init() {
	proxyTemplate = template.Must(template.New("proxyconfig").Parse(proxyTemplateString))

	proxyIPv4 = os.Getenv("COREOS_PUBLIC_IPV4")
	if proxyIPv4 == "" {
		panic("COREOS_PUBLIC_IPV4 not set")
	}

	proxyIPv6 = os.Getenv("COREOS_PUBLIC_IPV6")
	if proxyIPv6 == "" {
		panic("COREOS_PUBLIC_IPV6 not set")
	}
}

// Write writes the necessary files
func Write() (err error) {
	err = WriteCerts()
	if err != nil {
		return nil
	}

	return WriteConfig()
}

func certFilename(serviceName string) string {
	return haproxyCertsDir + "/" + serviceName + ".pem"
}

// WriteCerts writes each certificate in the service list
// to a file
func WriteCerts() (err error) {
	for _, s := range services {
		if s.Cert == "" {
			continue
		}
		f, err := os.Create(certFilename(s.Name))
		if err != nil {
			return err
		}
		_, err = f.WriteString(s.Cert)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Write the haproxy config file
func WriteConfig() (err error) {
	// Open file for writing
	tf, err := ioutil.TempFile("/tmp", "proxycfg")
	if err != nil {
		return err
	}
	defer os.Remove(tf.Name())

	err = proxyTemplate.Execute(tf, HAProxyConfig{
		IPv4:     proxyIPv4,
		IPv6:     proxyIPv6,
		Services: services,
	})
	if err != nil {
		return err
	}

	// Copy the new config into place
	err = shutil.CopyFile(tf.Name(), haproxyConfig, true)
	if err != nil {
		return err
	}

	return nil
}

// Reload tells haproxy to reload its configuration
func Reload() (err error) {
	conn, err := dbus.NewSystemdConnection()
	if err != nil {
		return err
	}

	_, err = conn.ReloadUnit("haproxy.cycore@"+instanceID, "ignore-dependencies", nil)

	return nil
}

const proxyTemplateString = `
global
	maxconn 4096
	quiet

defaults
	log	global
	mode	http
	option	http-server-close
	option	redispatch
	timeout	connect	5000
	timeout	client	50000
	timeout	server	50000

	stats enable
	stats uri /proxy-stats
	stats realm haproxy\ statistics

# -
# Frontends
# -
frontend public
	bind {{.IPv4}}:80
	bind {{.IPv6}}:80
	reqadd X-Forwarded-Proto:\ http

	# Bind aliases to backends
	{{range $service := .Services}}
		{{range $name := .Names}}
			use_backend {{$service.Name}} if { req_ssl_sni  {{$name}} }
		{{end}}
	{{end}}


frontend public_ssl
	bind {{.IPv4}}:443 ssl {{range $service := .Services}}{{if $service.Cert}} crt {{call $service.CertFile}} {{end}} {{end}}
	bind {{.IPv6}}:443 ssl {{range $service := .Services}}{{if $service.Cert}} crt {{call $service.CertFile}} {{end}} {{end}}
	reqadd X-Forwarded-Proto:\ https

	# Bind SNI indicators to backends
	{{range $service := .Services}}
		{{range $name := .Names}}
			use_backend {{$service.Name}} if { req_ssl_sni  {{$name}} }
		{{end}}
	{{end}}

# -
# Backends/Services
# -
{{range $service := .Services}}
backend {{$service.Name}}
	mode http
	cookie {{$service.Name}} insert indirect nocache
	option forwardfor
	balance roundrobin

	{{range $node := $service.Backends}}
	server srv{{index}} {{$node}}
	{{end}}
{{end}}

`
