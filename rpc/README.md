# Protocol

Following are the main changes that have been made to the zk protocol.

The high level changes are that servers are now accountless and that clients
use ephemeral identities. This essentially makes clients anonymous, provided
they uses tor to mask their IP addresses as they connect and communicate with
the server.

Clients determine the rendezvous point with other clients in a way that the
server can only track the absolute minimum amount of metadata.

All client to server communications are double encrypted using a TLS tunnel as
the outer layer and `NaCl secretbox` as the inner layer.

All client to client communications are encrypted with a double ratchet (a
third layer of encryption) with keys that only the clients have. The server
cannot read these messages. These messages wrap a new client to client
protocol.

For now, the client and server reuse the old initiate/acknowledge protocol that
is XDR encoded.  This should be reevaluated since XDR has some serious
drawbacks.

## Client/Server key exchange

The client server key exchange was gutted in favor of creating an anonymous
accountless client connection. In order to achieve this the server currently
has to identify itself in cleartext (inside the TLS tunnel) since the client
must know the server public key before connecting. This portion may have to be
rethought.

1. Client creates TLS tunnel to the server.
1. Client creates an ephemeral sntrup4591761 encapsulated shared key and send it to the server.
1. Server receives encapsulated shared key and decapsulates it.
1. Both the client and server use the shared key to read and write. In order to ensure no reuse of the nonce the nonce space is divided by two.
1. Server write key starts at `0` and client starts at `nonce_space/2`. Both keys count up. The nonce space is 24 bytes and thus is large enough to never be reused in a session. Note that the shared key is random and is generated every session.
1. Both client and server at this point write commands to the other side using the shared key and their respective nonce spaces. Every message that is sent increments the nonce.

## Routed messages

The new protocol uses routed messages vs account messages. The client generates
the rendezvous point (a collider that both clients can predict) and sends it
along with encrypted blob. Currently it uses and HMAC of the send header key
where the message number is the key to the HMAC. The server verifies that it is
indeed a random 32 byte number that is hex encoded and uses that as the
filename to drop the blob on the filesystem. Once a client retrieves a blob
from the filesystem it is deleted (this needs to happen after the client ACK at
some point). Once a client receives a blob it uses the normal client-to-client
ratchet to decrypt it. Inside the decrypted blob is the actual command that the
client receives from the other side.

This section is sparce because the rendezvous is currently a bit in flux and
has a mechanism built-in to switch between methods. The expectation is that
this will be modified as we run into ratchet errors and have to come up with
methods on how to work around that.

## Out-of-band invites

To get an initial ratchet key exchange kicked off the tool provides a method to
export and import a file with enough information to bootstrap that process.

The inviter creates a `PublicIdentityInvite` structure that contains their
entire public identity and keys. In addition this structure contains a 32 byte
field called `InitialRendezvous` which is used to hint the invitee where to
send their response. The invitee subscribes to the `InitialRendezvous` with the
server. The `PublicIdentityInvite` is JSON encoded and NaCl secret box
encrypted with a key derived from a trivial PIN.

1. Alice creates a `PublicIdentityInvite` with a strong random `InitialRendezvous`. The `PublicIdentityInvite` is JSON encoded and NaCl secret box encrypted. This blob is exported as a file which Alice must send to Bob out-of-band. Alice subscribes for notification at the `InitialRendezvous` route.
1. Bob receives the encrypted `PublicIdentityInvite` and decrypts it using the PIN provided by Alice. Bob is prompted to accept Alice's fingerprint and if accepted Bob kicks of the in-band portion of the ratchet key exchange by routing a `RMIdentityKX` command to the `InitialRendezvous`. The `RMIdentityKX` contains Bob's public identity and keys, an `InitialRendezvous` for Bob to subscribe to, and a half ratchet. Bob routes the `RMIdentityKX` to the server using the newly generated `InitialRendezvous` and  subscribes to it.
1. Alice receives the `RMIdentityKX` command and finalizes the ratchet. Alice routes a `RMKX` command using Bob's `InitialRendezvous` which contains the finalized ratchet. Alice subscribes to the HMAC of the send ratchet header key.
1. Bob receives the `RMKX` and subscribes to the HMAC send ratchet header key.

Notes: The HMAC of the send header key uses a counter as the key. This jumbles
the rendezvous for every command while remaining predictable. If a key has a
zero value, which happens once during key exchange, the next send header key is
used.

## Transitive invites

Consider the following situation: Alice and Bob are communicating and Bob
decides that Alice should talk to Charlie. Bob sends Alice a request to speak
to Charlie and facilitates delivery of key exchange material. Naturally, this
needs to be non-forgeable.

The process is as follows:
1. Bob sends Alice a `RMInvite` command with Charlie's public key
1. Alice responds to Bob with a `RMInviteReply` that contains an encrypted  `PublicIdentityInvite`. Since Alice received Charlie's public key she can use `sntrup4591761` to encapsulate a shared key between Charlie and her. That shared key is then used to encrypt the `JSON` encoded `PublicIdentityInvite` blob using NaCl secretbox.
1. Bob receives the `RMInviteReply` and forwards it to Charlie. Bob could try to decapsulate the shared key but will not succeed and therefore cannot decrypt the `JSON` encoded `PublicIdentityInvite` blob.
1. Charlie receives the `RMInviteReply` and decapsulates the shared key using his `sntrup4591761` private key. Charlie then uses the shared key to decrypt the NaCl secret box that contains the `PublicIdentityInvite`. At this point a ratchet key exchange can commence using the same exact mechanism as the out-of-band as described above.

Notes: there is an assumption that people can externally verify identity fingerprints or that the facilitator is trusted.

## Resetting the ratchet

Ratchets are brittle and break from time to time. In order to be able to
re-establish a ratchet a sideband for communication exists. This sideband is
dubbed `Emergency` and is a well known rendezvous point where the communication
mechanism does not use a ratchet but rather a sntrup4591761 generated key.

The rendezvous point is currently not smart and is simply called `reset` and
`reset.reply`. This will obviously break all kinds of things. This section will
be replaced once the rendezvous spec is finalized.

The `Emergency` commands are encrypted as follows: `[sntrup4591761 encapsulated key][nonce][encrypted emergency command]`.
Once the `encrypted emergency command` is decrypted it contains an embedded
command of which `reset ratchet` is one.

The reset procedure is as follows:
1. Alice sends Bob an `RMEReset` command on the emergency rendezvous. Alice deletes ratchet files of disk but not the identity.
1. Bob receives `RMEReset` command and deletes ratchets of disk and sends a normal `PublicIdentityInvite` encrypted reusing the sntrup4591761 shared key to Alice's emergency rendezvous.
1. At this point the process becomes the same as a out-of-band invite.

## Attack vectors

TBD

## To do
1. Receipts
1. First pass to serve data use case (ala FTP with price attached)
