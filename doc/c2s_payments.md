# Client-Server Payments

In order to reduce the possibility of spam and DoS attacks, client-server (C2S)
communications are predicated on Lightning Network payments.

The two main operations that the server provides to clients are charged:

- Subscription to a RV
- Sending routed messages

Before subscribing to a set of Rendezvous Point (RV point), clients must pay for
any previously unpaid-for RVs. The server has a specific, per-RV rate that is
charged. Each RV that is paid can be subscribed to any number of times for some
interval (currently hard-coded as 7 days).

Correspondingly, in order to send a Routed Message (RM) to be stored at the
server, clients must pay a fee charged per byte of the future RM.

RM sending and RV subscription can only happen sequentially: in order to send
(for example) two RMs in sequence, the first one needs to be paid then
delivered and acknowledged by the server before the next one can be paid and
sent.

To simplify the flow, every ack for one of the actions includes an LN invoice to
be paid for the next action so that the client can directly initiate the sending
process by performing the payment directly.


```
│Alice                               Server                              Bob │
├──────                             ────┬───                            ─────┤
│                                       │                                    │
│ rpc.GetInvoice                        │                                    │
├───────────────►                       │                     rpc.GetInvoice │
│ (action=sub)                          │                    ◄───────────────┤
│                                       │                      (action=push) │
│                                       │                                    │
│                    rpc.GetInvoiceReply│                                    │
│                  ◄────────────────────┤                                    │
│                    (invoice=ln...)    │   rpc.GetInvoiceReply              │
│                                       ├───────────────────►                │
│Pay LN Invoice                         │   (invoice=ln...)                  │
│                                       │                                    │
│                                       │                      Pay LN Invoice│
│rpc.SubRoutedMessages                  │                                    │
├─────────────────────►                 │                                    │
│  [rv1, rv2, ...]                      │                    rpc.RouteMessage│
│                                       │                  ◄─────────────────┤
│                   rpc.SubRMReply      │                       (rv = rv1)   │
│                ◄──────────────────────┤                                    │
│                 (next invoice=ln...)  │rpc.RMReply                         │
│                                       ├───────────────────►                │
│                                       │(next_invoice=ln...)                │
│                       rpc.PushRM      │                                    │
│                ◄──────────────────────┤                      Pay LN Invoice│
│                       (rv = rv1)      │                                    │
│                                       │                                    │
│                                       │                    rpc.RouteMessage│
│                                       │                  ◄─────────────────┤
│                                       │                       (rv = rv2)   │
│                                       │rpc.RMReply                         │
│                                       ├───────────────────►                │
│                        rpc.PushRM     │(next_invoice=ln...)                │
│                ◄──────────────────────┤                                    │
│                        (rv = rv2)     │                                    │
```

