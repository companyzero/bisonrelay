Bison Relay
===

[![Build Status](https://github.com/companyzero/bisonrelay/workflows/Build%20and%20Test/badge.svg)](https://github.com/companyzero/bisonrelay/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://pkg.go.dev/github.com/companyzero/bisonrelay)

Bison Relay (BR) is a suite of programs to enable private and secure
communications between any number of parties.

The server is oblivious to the contents of individual messages (i.e. all
messages are E2E encrypted) and Lightning Network payments are required in order
to send and receive messages.


## Building

Building the software in this repository requires Go version 1.18+. Proceed with
the standard method for building and installing Go binaries.

## Quick Start

The following assumes there is a suitable version of the Go toolset installed.

### CLI Client

The basic CLI client is available in the `brclient` package. It can be installed
and ran by with the following commands in a checked out copy of this repository:

```
$ go install ./brclient
brclient
```

If this is the first time the client is being executed, it will by default
go through the initial setup wizard and will create a file named
`~/.brclient/brclient.conf` to hold the config.

During the setup wizard, the user will be asked to create a
[dcrlnd](https://github.com/decred/dcrlnd) wallet if it can't provide connection
details to one. In this case, `brclient` will run an embedded LN wallet,
including with the on-chain services necessary to fully operate it. The client
will not be usable until the initial sync completes and the LN wallet is fully
operational.

Note that in order to send and receive messages with other users, the associated
LN wallet (either the embedded or an external one) **MUST** be actively managed
by the user, including by having active channels and enough outbound bandwidth
to be able to make the payments required by the server.

#### Basic Commands

The entire list of supported commands can be found by typing `/help` after the
client is fully setup. Further information about a command be obtained by typing
`/help <command>`.

- `/ln <subcommand>`: perform operations in the underlying LN wallet. Including:
  - `/ln info`: show info about the current LN wallet.
  - `/ln newaddress`: generate a deposit address for DCR that can be used to
    fund the on-chain wallet operations.
  - `/ln openchannel`: open an outbound channel to a target peer.
  - `/ln requestrecv`: request inbound capacity by having a remote node open a
    channel back to the local node (requires paying the remote node).
- `/invite <filename>`: generate an "invitation" file that can be sent to
  another user to start communicating with them.
- `/add <filename>`: accept the invitation to communicate with a user.
- `/msg <user> <message>` send a private message to a previously known user.

#### Client Automation

Automation (bots, integrations, etc) of a `brclient` instance can be done by
using the [clientrpc](clientrpc) interface.

#### Simple Store

More information about running a simple store can be found in
the [/doc](/doc/simplestore.md) subdir.

### Server

A private server can be executed by running:

```
$ go install ./brserver
$ brserver
```

The sample config file for a server install is available in the
[brserver.conf](/brserver/brserver.conf) file in this repository.

## Further Reading

More information about the internal architecture of bison relay can be found in
the [/doc](/doc/README.md) subdir.

## Verifying Binaries

For your security, we recommend that you verify binaries before running them.
Each release contains a manifest file with SHA-256 hashes for each released
binary. To ensure your downloads are authentic, you should verify that the
manifest file is signed by `release@decred.org`, and that your hashed binary
matches the manifest.

Detailed instructions can be found in the Decred Documentation:
[Verifying Binaries](https://docs.decred.org/advanced/verifying-binaries/).
New users should start there.

If you've already done this before and you still have the Decred Release keys
on your GnuPG keyring, the following shorthand instructions are provided as a
quick refresher:

1. Download:

   * The zip/tarball for your specific OS / architecture
   * The file manifest and hashes, ending in `-manifest.txt`
   * The signature for the manifest, ending in `-manifest.txt.asc`

2. Verify that the manifest was directly signed by the Decred project:

   ```
   $ gpg --verify <your manifest.txt.asc file>
   ```

   Example output:
   ```
   gpg: assuming signed data in 'decred-v1.5.1-manifest.txt'
   gpg: Signature made 01/29/20 15:17:58 Eastern Standard Time
   gpg:                using RSA key F516ADB7A069852C7C28A02D6D897EDF518A031D
   gpg: Good signature from "Decred Release <release@decred.org>" [unknown]
   gpg: WARNING: This key is not certified with a trusted signature!
   gpg:          There is no indication that the signature belongs to the owner.
   Primary key fingerprint: FD13 B683 5E24 8FAF 4BD1  838D 6DF6 34AA 7608 AF04
      Subkey fingerprint: F516 ADB7 A069 852C 7C28  A02D 6D89 7EDF 518A 031D
   ```

   If you see `Good signature from "Decred Release <release@decred.org>"`, then
   you're successful! You can trust that the `manifest.txt` came directly from the
   Decred project.

3. Verify that the hash of your downloaded zip/tarball matches the manifest hash:

   * Windows:

      * If you have [7-Zip](https://7-zip.org/) installed, simply open up Windows
      Explorer, right click on the file, mouseover `CRC SHA`, then click `SHA-256`.

      * `$ certutil -hashfile <your file> SHA256`

   * macOS

      * `$ shasum -a 256 <your file>`

   * Linux

      * `$ sha256sum <your file>`

   Example output:
   ```
   0c43caffa428cebb8a4d3c8efb2a341220fd1c232640ff3b4403ff67e1873e1a  decred-linux-amd64-v1.5.1.tar.gz
   ```
   
If your output hash matches the hash from the manifest, you're done! The binary
for your platform is now verified and you can be confident it was generated by
the Decred Project. It's safe to install the software.

## Disclaimer

**BR has not been audited yet.  Use wisely.**

## License

BR is licensed under the [copyfree](http://copyfree.org) ISC License.
