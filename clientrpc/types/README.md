# Bison Relay clientrpc types

This package defines the types used within the `clientrpc` protocol for Bison
Relay client automation.

The majority of the code in this package is automatically generated, based on
the definitions on the [clientrpc.proto](../clientrpc.proto) file.

End-users are usually interested in the definitions for the client side of the
services defined in the proto file. Following are links for documentation about
the currently available service clients:

  * [VersionServiceClient](https://pkg.go.dev/github.com/companyzero/bisonrelay/clientrpc/types#VersionServiceClient): Provides functions to fetch metadata about the clientrpc server (i.e. the underlying brclient).
  * [ChatServiceClient](https://pkg.go.dev/github.com/companyzero/bisonrelay/clientrpc/types#ChatServiceClient): Provides functions to interact with the chat services of the clientrpc server.


