# FR Client Library

# Testing

E2E testing requires a running server configured with ln payments. It assumes
the dcrlnd three-node tmux environment is running.

```
$ go run ./brserver -cfg client/e2e-legacy-ln-server.conf
$ go test -count=1 -run TestE2E -tags e2elegacylntest -v ./client/ | tee /tmp/out.txt
```


# Architecture

The diagram below shows the major layers of the client. Depicted is Alice
sending a message while Bob is receiving it.

```
                                                                                                                                                
                            Alice                      server                        Bob        
                                                                                                
                                                                                                
                        +----------+                                              +----------+  
                        |  Client  |                                              |  Client  |  
                        +----------+                                              +----------+  
                              |                                                         |       
                              |                                                         |       
                              |                                                         |       
                       +------------+                                            +------------+ 
                       | RemoteUser | . . . . . . . . . . . . . . . . . . . . .  | RemoteUser | 
                       +------------+                                            +------------+ 
                              |                                                         |       
                              |                                                         |       
                              |                                                         |       
                         +---------+             +-----------------+           +---------------+
                         |   rmq   | . . . . . . |  subscriptions  |. . . . . .   rdzvManager  |
                         +---------+             +-----------------+           +---------------+
                              |                           |                             |       
                              |                           |                             |       
                              |                           |                             |       
                     +-----------------+         +------------------+         +---------|-------+
          ---------->|  serverSession  | . . . . |  client session  | . . . . |  serverSession  |
         /           +-----------------+         +------------------+         +-----------------+
        /               /     |                           |                             |    
+-------------+        /      |                           |                             |    
| connKeeper  |<-------       |                           |                             |    
+-------------+           +-------+                   +-------+                     +-------+
                          |  net  |-------------------|  net  |-------------------- |  net  |
                          +-------+                   +-------+                     +-------+
```

Each layer has certain specific responsibilities and attempts to maintain
certain invariants for the other layers:

  - `net`: Lower-level network connection (currently TLS)
  - `connKeeper`: Attempts to keep an open connection to the server. Whenever a
    new connection is made, a new instance of `serverSession` is created
  - `serverSession`: Maintains the tag stack invariant (max inflight, non-acked
    messages to the server), encrypts/decrypts the wire msgs w/ the per-session
    key
  - `rmq`: outbound RoutedMessages queue; pays for each outbound RM before
    sending it, encrypts each RM according to its type (clear text, kx, ratchet)
  - `rdzvManager`: Rendezvous Manager, is the inbound RM queue; ensures the
    client is subscribed to the appropriate RV points in the server as needed
    and decides which `remoteUser` to call for every pushed RM
  - `RemoteUser`: holds the ratchet state for each user the client has completed
    a kx flow with and progresses it as RMs are sent and received. Also
    implements per-user higher level calls (send/receive PMs, files, etc)

Each layer is tipically implemented as a runnable state machine, where its
API is (usually) synchronous and concurrent-safe and actions are
performed in its `run()` call.

The main public-facing structures of this package are `Config` and `Client`.
Consumers (e.g. a GUI or CLI client app) setup a `Config` instance as needed,
create a new client instance and run it. The client will maintain its operations
until one of its subsystems fail (in which case `Run()` returns with an error)
or the context is canceled.
