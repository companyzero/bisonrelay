# P2P Messaging Protocol

Messages between two [already KX'd](p2p_kx.md) peers are made through an E2E
encrypted message system that uses the FR server as message repository.

To exchange a message, the sender stores its encrypted contents on the server,
addressed to a so called "rendezvous point" (RV point). This RV point is a
random-looking 32 byte sequence.

A receiver then queries the server for the contents of that same RV point, and
the server (assuming it in fact stored it) will relay it.

The RV points are derived from the [double
ratchet](https://signal.org/docs/specifications/doubleratchet/) that is
established between the peers at the end of the key exchange (KX) protocol.
Therefore, after each exchange messaged, the RV points are rotated such that no
two messages are sent to the same point and the points themselves are
uncorrelated from the point of view of an external observer.

The RV points are derived from the corresponding send or receive header keys,
concatenated with the current message count. For the receiving end, and
additional rv point is derived by using the previous receive header key and
message counts to reduce the chance of missing messages in case of de-sync of
message counts:

```
send_rv = blake256(send_header_key, send_msg_count)
recv_rv = blake256(recv_header_key, recv_msg_count)
drain_rv = blake256(prev_recv_header_key, prev_recv_msg_count)
```

When sending a message, a client sends a `rpc.RouteMessage` command to the
server, addressing it to its current `send_rv`, along with the
double-ratchet-encrypted message itself.

The counterparty on the other hand, will send a `rpc.SubscribeRoutedMessages` to
the server including its `recv_rv` and `drain_rv` points. If the server has
already stored the message, then it will directly relay it to the receiving
peer. If the message arrives in the future (while the receiving peer is
connected to the server), then it will relay the message at that moment.
