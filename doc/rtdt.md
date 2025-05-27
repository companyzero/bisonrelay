Real Time Datagram Tunneling Bison Relay Protocol

Last Updated: 2025-01-17

# Goal

Enable realtime audio/video/data streams among BR users.

# Executive Overview

Realtime AV (audio/video) protocols should preferably use UDP (due to ability to
handle dropped and reorged packets). They should preferably allow some form of
multicast (to improve bandwidth usage). This requires designing a new protocol
in BR ecosystem.

This protocol will be called Real Time Datagram Tunneling (RTDT-BR or just RTDT)
in this document.


                        Routed Messages
          ..............................................
          │                                            │
┌─────────┴───┐         ┌─────────────┐         ┌──────┴──────┐
│             │  PRPC   │             │  PRPC   │             │
│  BR Client  ├─────────┤  BR Server  ├─────────┤  BR Client  │
│             │         │             │         │             │
└─────────┬───┘         └─────┬───────┘         └──────┬──────┘
          │                   :                        │       
          │                   : Encrypted              │       
          │RTDT               : Cookie                 │RTDT   
          │Transport          : Data                   │Transport
          │Protocol           :                        │Protocol
          │           ┌───────┴─────────┐              │       
          │           │                 │              │       
          └───────────┤   RTDT Server   ├──────────────┘       
                      │                 │                      
                      └─────────────────┘                      

A set of peers exchange messages in RTDT "sessions" ("channels", "rooms", etc).
A session is identified by an RV point. The RTDT server binds sessions to IP
addresses of participants. BR server manages session metadata (size of session,
lifetime, permissions, etc).

PRPC is the existing brclient <> brserver protocol and is used to manage
sessions.

RTDT Transport Protocol is the custom client/server, encrypted UDP transport 
protocol which routes E2E encrypted realtime date through the intermediate RTDT
server.

Data between BR serevr and RTDT server is exchanged through the use of opaque,
encrypted cookies. These are created by BR server and validated by RTDT server
to verify session and permission data.

Only the realtime data (actual audio/video/data contents) flows through the RTDT
server. Session management (including creation, modification, moderation and
termination) are all done through the standard PRPC protocol that already exists
between brclient and brserver.

The data is E2E encrypted with a key shared by participants for a given session.
This allows participation by both KX'd and Un-KX'd peers in a session (with
inner signatures authenticating origin).

Payment is sent by each participant, proportionally to the size of the session
(larger sessions require larger payment). Payment is only done and tracked by
sent data: senders pay for data but receivers do not. Senders pre-pay for a
certain amount of data to be sent ("allowance"), then data sent is subtracted
from this allowance until that is zeroed (or sender performs another payment).

RTDT is NOT a P2P system: data is relayed through the RTDT server between
participants in a multicast fashion: within a session, one message sent by a
client is replicated N times (once for each of the other participants). This
can be considered an application-level multicast (as opposed to IP-level
multicast).



Client A                    BR Server               RTDT Server         Client B
  │                            │                       │                     │  
  │                            │                       │                     │  
  │     Create Session         │                       │                     │  
  ├───────────────────────────►│                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │     Share Session Info     │                       │                     │  
  ├────────────────────────────┼───────────────────────┼────────────────────►│  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │        Join Session        │                       │                     │  
  ├────────────────────────────┼──────────────────────►│                     │  
  │                            │                       │                     │  
  │                            │                       │     Join Session    │  
  │                            │                       │◄────────────────────┤  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │          RT Data           │                       │                     │  
  ├────────────────────────────┼──────────────────────►│                     │  
  │                            │                       │                     │  
  │                            │                       │       RT Data       │  
  │                            │                       ├────────────────────►│  
  │                            │                       │                     │  

# Alternative Existing Technologies Considered

- WebRTC 
  - Works on browser
  - Works for both P2P and C2S transmission
  - Large API surface
  - Requires additional server software anyway
  - Requires bolting E2E encryption anyway
  - Requires bolting payments anyway

- TURN
  - Only deals with one side of UDP connection
  - Does not provide multicast ability


# Current design features

Note: Not all design features have been fully written.

- Ability to create persistent sessions
- Ability to join/leave realtime session multiple times
- Multiplex sessions through a single UDP connection
- Multiplex streams (audio/video) inside a session
- Ability to manage session participation:
  - Peer can join only when they have an "appointment cookie"
  - Peer can be kicked from session and prevented from re-joining
  - Owner + Admins
- Sized sessions (max nb of peers inside session)
- Payment for relay of data
  - Payment amount is based on size of session
- Ability to move to new session
  - When session will expire or needs resizing
  - May require user confirmation in UI


# Overview of Design Features

## Session IDs

The full session ID is similar to an RV point: a random 32 byte value. This is
meant to be hard to guess. This full session ID is derived as the hash of random
client and server nonces.

To improve bandwidth requirements, realtime data is id'd by a short id: a uint32
(4 bytes). This identifies both a session and a specific client within a
sesssion. There is no structure to this value (it should be a random number).

## Session Size

Sessions have a max number of participants allowed ("size"). This is defined
during session creating and cannot be modified (because payment for sending data
to the session is proportional to session size).

If the size needs to be changed, then a new session must be created and all
clients should be signalled to leave the old session and join the new one.

## Encryption and Authentication

Client to RTDT server communication is encrypted through the use of standard
SNACL (XSalsa20+Poly1305) message encryption. Encryption keys for this are
derived by using the SNTRUP4591761 protocol, with long-term RTDT server keys
shared by the corresponding BR server.

End-to-End (i.e. client to client) encryption is further achieved by using
XChacha20-Poly1305. E2E encryption keys for "publishers" (session participants
that are allowed to send data) are shared through standard BR messages.

Authentication of source data is further optionally enabled by allowing
inclusion of a signature on the inner packet. This signature is done by the
source client's long term signing key.

The publisher keys are unique per session, to ensure cryptographic assumptions
about max message sizes are respected and provide forward secrecry.

## Opaque Cookies

Session management and data sharing between BR server and RTDT server is done
through the use of opaque cookies. Three cookies are defined:

- Session Cookie: requested by the session's owner, defines the basic session
  metadata.
- Appointment Cookie: obtained by the session's owner before inviting a remote
  client to join the session.
- Join Cookie: obtained by a client just before joining the live RTDT session
  and to refresh its publishing allowance.

Cookies are encrypted with a secret key, shared between BR and RTDT servers. Use
of these opaque cookies removes the need for an online, continuous communication
between BR and RTDT servers.

The following flow shows how cookies are created, shared and used. Client "A" is
considered the owner of the session. The owner obtains a session cookie and uses
it to get appointment cookies. The appointment cookie is used (by client "B") to
obtain a limited-lifetime join cookie. The join cookie is used to join the
session in the RTDT server.


Client A                    BR Server               RTDT Server         Client B
  │                            │                       │                     │  
  │     CreateRTDTSession      │                       │                     │  
  ├───────────────────────────►│                       │                     │  
  │          Ack               │                       │                     │  
  │◄───────────────────────────┤                       │                     │  
  │       (session cookie)     │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │  GetAppointmentCookies     │                       │                     │  
  ├───────────────────────────►│                       │                     │  
  │     (session cookie)       │                       │                     │  
  │                            │                       │                     │  
  │          Ack               │                       │                     │  
  │◄───────────────────────────┤                       │                     │  
  │     (appointment cookie)   │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │      Session Info*         │                       │                     │  
  ├────────────────────────────┼───────────────────────┼────────────────────►│  
  │     (appointment cookie)   │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │  AppointRTRTServer  │  
  │                            │◄──────────────────────│─────────────────────│
  │                            │                       │(appointment cookie) │  
  │                            │                       │                     │  
  │                            │                       │    Ack              │  
  │                            ├───────────────────────┼────────────────────►│  
  │                            │                       │   (join cookie)     │  
  │                            │                       │                     │  
  │                            │                       │                     │  
  │                            │                       │    CmdJoinSession   │  
  │                            │                       │◄────────────────────┤  
  │                            │                       │   (join cookie)     │  
  │                            │                       │                     │  
  │                            │                       │   Ack               │  
  │                            │                       ├────────────────────►│  

_`*`: "Session Info" is an abstraction over the C2C invite and session metadata
sharing process._

## Session Security

Joining a session on the RTDT server (i.e. binding an IP address as a receiver
and potential sender of session data) requires presenting the "join cookie".
This cookie ensures only authorized clients will send or receive data from the
server for the session.

Internally (after decryption), a join cookie contains enough metadata to
uniquely identify the session. It also contains the publishing allowance for
that session participant (i.e. how much it paid to route data through data
session).

The following information is used to uniquely identify a session:

- Session RV
- Server Secret Nonce
- Owner Secret Nonce
- Size of Session

Thus, if a client presents a cookie with any change to any of these parameters,
it won't be able to communicate with other remote peers. This allow a session's
owner to "rotate" the session (by changing its "owner secret nonce") and sending
new cookies to participants.


## Session Management Permission

Session metadata is shared through standard routed messages between BR clients.
These follow the same pattern as GCs (group chats): the "owner" may set some
identities as "admins", and then remote clients should accept updates from both
the owner and admins.

From the POV of the RTDT server, some members have an `is_admin` flag set in
their join cookies which allows them some special operations inside that session
(such as kicking another member).

## Publishers and Spectators Capability

Peers may or may not route data through the RTDT server after joining it. If
they do send data, they are called "publishers". If they only receive data, they
are called "spectators".

Publishers E2E encrypt their data with an individual encryption key. This key is
shared along with the session metadata, through routed messages, with all
members (from either an owner or admin).

Spectators are invited to join the session but may only receive data, not send
it. This requires fewer payments to maintain the live session.

## Payments

The following actions require payment. Most of these actions use the format of
requiring a minimum payment plus a per-user fee.

The rates for these actions are parametrized and clients should handle allowing
policy updates by the server.

### Create/Manage Session

In order to create a session (i.e. obtain a session cookie), the admin must
perform a payment (minimum amount + per-user fee).

### Get Appointment Cookie

In order to invite users to join a session, a session admin must obtain an
appointment cookie (by presenting the previously created session's cookie). 

This is a server-batched call, with a minimum payment + per-user fee (when
requesting multiple cookies for multiple users simultaneously).

### Join Session

In order to join a session, a client must obtain a join cookie (by presenting an
appointment cookie previously obtained by an admin).

There is a minimum amount paid (which is the only payment required by
spectators) and then a per-user-megabyte paid by publishers (see next session).

### Send Data

In order to send data to a session, a client must perform a payment at a rate
that is proportional to the size of the session (independently of whether there
are other clients connected or not). This is called the "session payment rate".

In order to guarantee latency for realtime data, this cannot be made to be one
payment per data packet. Therefore, this needs to be done on pro-rated fashion:

A client that wishes to send some data, should perform a payment at a given
session payment rate for a certain amount of megabytes. Once payment completes,
this will be added to the client's available data allowance.

When sending data, the RTDT server deducts from the client's allowance until
that is zeroed. Once it is zeroed, the server will stop relaying client data.

Clients should estimate the size of the allowance that will be required, based
on the type of data they will send (audio/video/other), the sending rate (e.g.
codec used and bitrate configured) and payment latency (a few seconds) in order
to keep their allowance at a level where they won't be unnecessarily blocked.

Any remaining balance is NOT returned to clients. The payment rate should be
selected to be small enough for this not to be a problem.

The payment rate should be defined in units of milliatoms per megabyte per
participant of the session.

## Server Metadata and Privacy

The current design exposes the following metadata to the combination of BR and 
RTDT servers:

- IP Addresses involved in a single session
- Number of messages exchanged (total, per session, per client/session)
- Size of messages exchanges (total, per session, per client/session)
- Role of participants in session (whether publisher or purely spectator)
- Session Metadata
  - Session RV
  - Clients joined/left/kicked


# Concrete Message Flows

A few selected message flows.


