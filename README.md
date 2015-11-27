# ccproxy

[![Docker Repository on Quay](https://quay.io/repository/cycore/ccproxy/status "Docker Repository on Quay")](https://quay.io/repository/cycore/ccproxy)

An etcd-backed haproxy sidekick.

Features:
  * SNI (multiple certs on a single IP)
  * etcd-based configuration and certificate storage, with change detection
  * manages restarts of haproxy on configuration change (via systemd)

See the [Unit file](ccproxy@.service) for usage information.

## Frontends

Frontends use the primary public IP addresses for the hosting node, registering them
with the DNS service.  The record ids for the created DNS records are stored in etcd
in the key `/cycore/proxy/dns/<instanceId>` so that these records may be updated or
deleted at a later time.

## Backends

Backends are updated based on the information registered by `gliderlabs/registrator`,
pulled from the name referenced in `/cycore/proxy/services/<serviceName>`.  In other words, the
actual service needs to be executed with the environment variable `SERVICE_NAME` set
corresponding to the `/cycore/proxy/services/<serviceName>` key.

## DNS Names

One or more DNS hostnames must be specified for haproxy to respond to. 

## Ports

For now, only ports 80 and 443 are used, and there is no support for specifying any
others.

## Certificate/TLS

The presence or absence of a TLS certificate bundle in `/cycore/proxy/services/<serviceName>/cert`
determines whether to offer SNI redirecting and TLS termination for the service.  The
value of that key should be the complete bundled `.pem` file.
