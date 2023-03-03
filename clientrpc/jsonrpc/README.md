# JSON-RPC 2.0 clientrpc transport

`jsonrpc` package implements [JSON-RPC
2.0](https://www.jsonrpc.org/specification) clients and servers for automating a
Bison Relay client activities, such as writing bots, interfacing with third
party systems, and so on.

The server side of this package runs inside BR clients (such as
[brclient](/brclient)) while the client side may be used by other applications,
such as bot daemons, to connect to and perform BR-related operations (such as 
sending and receiving PMs, GC messages, etc).

End-users are generally interested in the client side of this package.
Currently, a single client mode is provided, which uses a websockets-based
transport. This transport allows both unary requests and client-initiated server
streams (requests where the server may stream data back to the client at
arbitrary times).

## Example Client

Following is the simplest example for creating a new client, connecting to a
brclient instance with the JSON-RPC enabled and using client certficate
authentication. Error handling is omitted for brevity:

```go
	c, _ := jsonrpc.NewWSClient(
		jsonrpc.WithWebsocketURL("wss://127.0.0.1:7676"),
		jsonrpc.WithServerTLSCertPath("/path/to/rpc.cert"),
		jsonrpc.WithClientTLSCert("/path/to/rpc-client.cert",
        "/path/to/rpc-client.key"),
	)
    go c.Run(context.Background())
	vc := types.NewVersionServiceClient(c)
    res := &types.VersionResponse{}
	_ = vc.Version(context.Background(), nil, res)
    fmt.Println(res)
```

## Example curl call

```shell
$ curl \
    --cert ~/.brclient/rpc-client.cert \
    --key ~/.brclient/rpc-client.key \
    --cacert ~/.brclient/rpc.cert \
    --data-binary '{"jsonrpc":"2.0","id":"dummy_id","method":"VersionService.Version","params":{}}' \
    https://127.0.0.1:7676/index
```

