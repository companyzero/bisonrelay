# P2P Key Exchange Protocol

The P2P Key Exchange protocol is used to establish a [double
ratchet](https://signal.org/docs/specifications/doubleratchet/) between two
users, such that individual messages between them are encrypted in a secure
way.

The P2P KX protocol is invoked in three different contexts:

- Manually, for initial setup of comms between two previously unkonwn users.
- Manually or automatically, to reset the ratchet whenever comms between the
  users fail ("busted ratchet" situation).
- Automatically, during an auto-kx procedure (e.g. after joining a GC).

The main difference between these contexts is how the initial message is
delivered from one user to the next.

The following is an overview of the protocol, for two (mutually) new users:


```
        Alice                                               Bob
       ───────                                             ─────

 msg1 = rpc.OOBPublicIdInvite
        ⤷ Alice.PubKey
        ⤷ initialRV
        ⤷ Alice_ResetRV

                                  msg1
                          ──────────────────────►
                              (out-of-band)

                                                msg2 = rpc.RMOHalfKX
                                                        ⤷ Bob.PubKey
                                                        ⤷ ratchet.KeyExchange
                                                        ⤷ step3RV
                                                        ⤷ Bob_ResetRV
                                      c1,k1 = sntrup.Encapsulate(Alice.PubKey)
                                                  blob2 = snacl.Seal(msg2, k1)


                                c1 ; blob2
                          ◄──────────────────────
                              (rv = initialRV)


 k1 = sntrup.Decapsulate(c1, Alice.PrivKey)
 msg2 = snacl.Open(blob2, k1)
 msg3 = rpc.RMOFullKX
         ⤷ ratchet.KeyExchange
 c2,k2 = sntrup.Encapsulate(Bob.PubKey)
 blob3 = snacl.Seal(msg3, k2)

                                c2 ; blob3
                          ──────────────────────►
                               (rv = step3RV)


                        Double Ratchet Established
```

Note that in this case, msg1 is sent out-of-band in plain text, from Alice to
Bob. Some third party messaging system must be used for this exchange, such that
the users are mutually assured to be talking to the intended recipient (for
example, they are exchanging data in-person via USB stick or through an E2E 
encrypted chat system).

## Ratchet Resets

When a ratchet reset is requested from either user, the initial
`rpc.OOBPublicIdentityInvite` is directly encrypted via the same `strup+snacl`
scheme using the other party's known public key and sent to the previously
agreed upon corresponding reset rv.

For example, if Alice wishes to request a ratchet reset, it encrypts a new 
invite and sends it to the `Bob_ResetRV` (instead of having to rely on an OOB
channel).

## Auto KXs

Auto-KXs happen whenever the KX process is initiated automatically. This is
usually done through the use of transitive requests: a third party helps
connect the two users by relaying the initial invite message between them.

For example, whenever joining a new group chat, a new user will attempt to
perform the KX procedure with every previously unknown member of the GC. To do
so, it will request the GC admin introduce them to the members by having the
admin relay an `OOBPublicIdentityInvite`.
