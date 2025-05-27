# RTDT Null Client

Stress test RTDT.

Prefer doing this just after deploying (and before actual users are using the
server) for best results.

## `rtnullclient` Usage

`rtnullclient` simulates an RTDT client. It only runs the RTDT protocol, so all
the other BR-related functionality is not present. It only uses the random data
stream (`rpc.RTDTStreamRandom`).

It works by simulating "bursts" of packets of a given inter-packet interval and
size. This is similar to what voice/video chats do (e.g. 1 packet of ~200 bytes
every 20ms for voice).

It can multiplex multiple bursts, sessions and connections to allow various
test scenarios.

Tests can be performed using one `rtnullclient` instance (on the same machine as
`brrtdtserver` or not) or multiple instances (e.g. Alice and Bob).

Scenarios are triggered by hitting the admin endpoints (`rtnullclient` runs an
admin interface in an embedded HTTP server).

## Test Conditions

There are different testing conditions/options/matrix that can be setup on
`brrtdtserver` and `rtnullclient`, depending on which scenarios you want to
benchmark.

The main testing configuration options are:

- Whether to use client/server encryption or not.
- Whether to use E2E (i.e. client-client) encryption or not.
- Whether to use E2E authenticated packets or not.
- Whether to require join cookies from clients or not.

Each option adds a little bit of overhead but makes the test more realistic
(compared to what brclient uses). OTOH, when stress testing the server itself,
E2E functions aren't really meaningful (because they are _client_ processes), so
they can be disabled to improve client performance.

Roughly speaking, E2E encryption tests the equivalent of
`RMRTDTSessionPublisher.PublisherKey` (i.e. encrypts packets between clients
using a symmetric key). E2E authentication tests that packets are signed with
the client's `PublicIdentity.SigKey`. See `rpc.RTDTDataPacket` data structure
for the implementation and mix/matching of E2E encryption.

## Test Supervision

`brrtdtserver` and `rtnullclient` both support running Prometheus endpoints.
Setup a Prometheus+Grafana install to monitor using that.

Both tools also support running the standard Go profiler.

`brrtdtserver` has a `statsinterval` config option, which can be used to set a
rate for logging basic stats.

Finally, another option for supervision is using standard OS and network tools.

## Test Source and Bottlenecks

One important consideration is _where_ to run `rtnullclient`, because that will
generally define which bottleneck you're testing.

If you run it on the server machine itself (wherever `brrtdtserver` is running),
then you'll be testing CPU, memory and OS networking stack capacity. Note that
in this case, you'll lose some performance from the test (because `rtnullclient`
itself will be using some resources).

Running it on different machines inside the same physical network, you'll be
testing your lower level network setup (network interfaces, layer 2 switch
performance, etc).

Finally, running it across the internet will be testing your entire network
layout (ISP throughput, upstream links, etc). On production deployments, this
will usually be your actual bottleneck.

There is no right or wrong answer here, different source locations test
different bottlenecks. Addressing one bottleneck (for example, upgrading layer 2
switching capacity) will necessarily move the bottleneck somewhere else. Tests
across all levels (and across time, software evolution and deployments) may be
necessary to identify the current bottlenecks.

# General Test Procedure

The _general_ test procedure is as follows:

- Setup Prometheus+Grafana (optional, for monitoring)
- Configure `brrtdtserver`
    - Set `privkeyfile` if you want to test client/server encryption
    - Set `cookiekey` if you want to test cookie decryption and allowance validation 
    - Set kernel buffer sizes (see [brserver README](/brserver/README.md))
- Run `brrtdtserver`
- Configure `rtnullclient`
    - Set `basepeerid` (each client should have a different one)
    - Set `serveraddr` and `serverpubkey` (if testing client/server encryption)
    - Set `enablee2e` and `enablee2eauth` according to your test scenario
    - Set `listen` with an address for the admin interface
- Run `rtnullclient`
- Hit `/batch` to trigger a batch of connections/sessions/bursts

# Single-Client Test Scenarios

The following concrete test scenarios run a single instance of `rtnullclient`.
It could be run on the same machine as `brrtdtserver` or not.

<details><summary>Example `rtnullclient.conf`</summary>
```
serveraddr = 127.0.0.1:7943
serverpubkey = /home/user/.brrtdtserver/server.key.pub
cookiekey = 0000000000000000000000000000000000000000000000000000000000000000 

listen = 127.0.0.1:7000
readroutines = 2
basepeerid = 1
enablee2e = 0
enablee2eauth = 0

[log]
debuglevel = debug

[debug]
profile = 127.0.0.1:9191
```
</details>

Run it with:

```
go run ./rtdt/internal/rtnullclient -cfg rtnullclient.conf
```


## Initial Smokescreen Test

This runs a single session between different connections of the client. In
practice, this will create 2 `rtdtclient` instances, use 2 different sockets to
connect to the server, then send data from one connection to the next.

Simulate listening peer:

```
curl "http://127.0.0.1:7000/batch?count=1&peer=1"
```

Simulate active peer:

```
curl "http://127.0.0.1:7000/batch?count=1&peer=2&addSpeech"
```

**Note**: make sure to use a different `peer=` argument on each call to force
different peer IDs and connections to the server.


## 1k Voice Sessions

This runs 1000 voice sessions between peers. By default, 20 sessions are
multiplexed per connection per peer (query parameter `sessPerConn` controls
that).

```
curl "http://127.0.0.1:7000/batch?count=1000&peer=1"
curl "http://127.0.0.1:7000/batch?count=1000&peer=2&addSpeech"
```

## Flood 100k Packets/Second

This attempts to flood around 100K packets per second (~10k packets/second
per session), sending 10 packets every 1 millisecond (over each connection):

```
curl "http://127.0.0.1:7000/batch?sessPerConn=1&count=10&peer=1"
curl "http://127.0.0.1:7000/batch?sessPerConn=1&count=10&peer=2&flood10k"
```

# Multi-Client Test Scenarios

On the following concrete test scenarios, "Alice" and "Bob" are two
`rtdtnullclient` instances. They may be running on the same machine or not. The
only requirement is that their `basepeerid` config be different (for example,
Alice should have `basepeerid=1` and Bob should have `basepeerid=2`).

If they are on the same machine, then they must also have different `listen`
addresses. For presentation purposes, we will determine that Alice has
`listen=127.0.0.1:7000` and Bob has `listen=127.0.0.1:8800`.

Create two files (`alice.conf` and `bob.conf`), then use
[rtnullclient.conf](rtnullclient.conf) as basis for the configuration. 

<details><summary>Example `alice.conf`</summary>
```
serveraddr = 127.0.0.1:7943
serverpubkey = /home/user/.brrtdtserver/server.key.pub
cookiekey = 0000000000000000000000000000000000000000000000000000000000000000 

listen = 127.0.0.1:7000
readroutines = 2
basepeerid = 1
enablee2e = 0
enablee2eauth = 0

[log]
debuglevel = debug

[debug]
profile = 127.0.0.1:9191
```
</details>

<details><summary>Example `bob.conf`</summary>
```
serveraddr = 127.0.0.1:7943
serverpubkey = /home/user/.brrtdtserver/server.key.pub
cookiekey = 0000000000000000000000000000000000000000000000000000000000000000 

listen = 127.0.0.1:7000
readroutines = 2
basepeerid = 2
enablee2e = 0
enablee2eauth = 0

[log]
debuglevel = debug

[debug]
profile = 127.0.0.1:9292
```
</details>


Run them with:



```
go run ./rtdt/internal/rtnullclient -cfg <name>.conf
```

## Initial Smokescreen Test

Validate your entire setup is working. This adds a single voice stream from
Alice to Bob.

On Alice:

```
curl "http://127.0.0.1:7000/addSpeech?sess=1"
```

On Bob:

```
curl "http://127.0.0.1:8800/join?sess=1"
```

Repeat on Alice and Bob (or switch around) to add more streams.


## 1k Voice Sessions

This runs 1K voice sessions from Alice to Bob using 20 sessions per connection. 

On Alice:

```
curl "http://127.0.0.1:7000/batch?count=1000&sessPerConn=20&addSpeech"
```

On Bob:

```
curl "http://127.0.0.1:8800/batch?count=1000&sessPerConn=20"
```

Note the above creates half-duplex sessions (Alice sends to Bob). To create full
duplex sessions (both Alice and Bob are sending), add `&addspeech` to Bob's
command.


