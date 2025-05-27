# RTDT Server

This is the realtime server for Bison Relay.

> [!IMPORTANT]
> UDP buffers are small by default on most OS installs. See the section below to
> increase them, otherwise you may get MANY packet drops.

# UDP Socket Buffers

The BR RTDT protocol uses UDP for data. This means sizing the kernel/OS buffers
on the server is *very* important to reduce chance of packet drops. Set those
buffers _before_ starting `brrtdtserver` (or restart it if you changed the OS
buffer sizes).

`brrtdtserver` can track and report the buffer size and packet drops on Linux,
_as long as you configure individual listen interfaces_ in `brrtdtserver.conf`.
That is, instead of using `listen = 0.0.0.0:7942`, use `listen =
127.0.0.1:7942,10.20.30.40:7942,...` so that the server binds on each individual
interface.

On OpenBSD, only the error rate (dropped packets) can be determined.

Other OSs are not supported.

The actual buffer size that should be used will depend on many factors (mainly
the lower level network throughput and the server hardware). Some
experimentation (along with stress testing using `rtnullclient` will be
required).

## Linux

```
# 33554432 == 32 MiB

WBZ=33554432
RBZ=33554432

sysctl -w net.core.rmem_max=$RBZ
sysctl -w net.core.rmem_default=$RBZ

sysctl -w net.core.wmem_max=$WBZ
sysctl -w net.core.wmem_default=$WBZ
```

## OpenBSD

```
# 33554432 == 32 MiB

sysctl net.inet.udp.{recvspace,sendspace}=33554432
```


