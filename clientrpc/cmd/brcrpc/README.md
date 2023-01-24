# brcrpc

Command `brcrpc` can run `clientrpc` commands on an underlying server (for
example, a running `brclient` instance).

## Installation

This requires a recent Go version.

```
$ go install github.com/companyzero/bisonrelay/clientrpc/cmd/brcrpc@latest
```

## Running

```
$ brcrpc -h
$ brcrpc -url wss://127.0.0.1:7676 VersionService.Version
```
