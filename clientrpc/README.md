# ClientRPC

`clientrpc` is a module that allows connecting to a running Bison Relay Client
instance (for example, a running `brclient`) through its automation RPC
endpoint, so that bots and other tools may be written.

A command line tool ([brcrpc](cmd/brcrpc)) is provided for quick testing and
debugging of the `clientrpc` interface.

The [examples](examples) dir shows how to write simple daemons that connect to
a brclient instance to perform basic automation tasks.

## Running brclient with the RPC endpoints

In order to connect to a running `brclient` instance, its clientrpc endpoint
must be enabled (it comes disabled by default). This must be configured in the
appropriate `brclient.conf` file (`~/brclient/brclient.conf` in unix-like
platforms):

```
[clientrpc]
jsonrpclisten = 127.0.0.1:7676
```

After restarting `brclient`, the following like should be displayed in the log:

```
[INF] RPCS: Listening for clientrpc JSON-RPC requests on 127.0.0.1:7676
```

## Authentication

Authentitcation for the clientrpc endpoint is currently based on TLS client
certificates. This allows mutual authentication between the server and client,
reducing the likelihood of unauthorized use of the API.

After enabling the `clientrpc` interface on `brclient`, a set of certificates
is created (by default, in the corresponding `~/.brclient` dir):

  - `rpc.cert`: Is the certificate for server the server side TLS connection.
  - `rpc.key`: Is the corresponding key for server-side encryption.
  - `rpc-ca.cert`: Is the CA certificate for the corresponding client
    certificates.
  - `rpc-client.cert`: Is the client certificate that clients must use to
    connect to the server.
  - `rpc-client.key`: Is the corresponding private key.

The client certificate, client key and client CA files must be used when
connecting to the `brclient` instance.


## Service Definitions

The services available through the `clientrpc` interface are defined in this
package's [clientrpc.proto](clientrpc.proto) file. This file is used by various
generators to create concrete types for writing both the server and client sides
of the `clientrpc` interface.

## Transports

The following transports are currently available:

### JSON-RPC 2.0

[JSON-RPC 2.0](https://www.jsonrpc.org/specification)-based transport. This
supports bidirectional communication over websockets, for both client-side
requests and server streams.

Messages are encoded as JSON objects, server-side stream events are encoded as
JSON-RPC notifications (i.e. requests without an id).

Package [jsonrpc](jsonrpc/) contains the Go implementation for this transport.
