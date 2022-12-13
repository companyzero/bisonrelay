# Client-Server Session Protocol

This protocol is used to establish the encrypted connection between a client and
the FR server.

Sessions run with two encryption layers: an "outer" TLS layer, plus an "inner"
encryption scheme based on the standard snacl.Seal() (XSalsa20+Poly1305).

The following diagram shows the rough sequence for establishing a C2S session:


```
     Client                                                     Server
    ────────                                                   ────────
                             TLS Connection
                    ────────────────────────────────►
                       rpc.InitialCmdIdentify
                    ────────────────────────────────►
                           zkid.PublicIdentity
                    ◄────────────────────────────────
                          rpc.InitialCmdSession
                    ────────────────────────────────►

c1, k1 = sntrup.Encapsulate(server.pubkey)

                                   c1
                    ────────────────────────────────►

                                    k1 = sntrup.Decapsulate(c1, server.privkey)

                   ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼
                       From this point on, msgs are
                       encrypted with
                       secretbox.Seal(data, k1, writeSeq)
                   ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼ ▼


                            rpc.SessionCmdWelcome
                    ◄────────────────────────────────

                     Client-Server Session Established!
```

Note that the client generates and encrypts a random shared key `(c1,k1)`, sending
`c1` to the server. The server decrypts `c1` (into `k1`) using its long-lived
private key and can then send and receive messages using the session.
