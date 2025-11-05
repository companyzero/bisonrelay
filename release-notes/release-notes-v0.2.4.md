# bisonrelay-v0.2.4

See bisonrelay-v0.2.4-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.

## Contents
* [Bison Relay GUI](#bruig-v024)
* [brclient](#brclient-v024)



# Bison Relay GUI (bruig)

* We're please to finally release our Real Time Datagram Tunneling (RTDT) Bison Relay Protocol. Users may now create RTDT sessions with numerous users (currently suggested for advanced users), or they can do simple instant calls by clicking the 'phone icon' at the top of their direct chat window.  This instant call aims to replicate the same functionality found within numerous other communication applications.

  For a comprehensize overview of how our RTDT system works you can read about it [here](https://github.com/companyzero/bisonrelay/blob/master/doc/rtdt.md).  


## Improvements

* Improve send and attach file UX.

* Clean up UI/UX of the user search/addressbook panel and functionality.

* Save chat embedded files to disk. 

* Save audio files (ogg) to disk after recording.

* Add attachment and emojis to Feed comments and replies.

* Properly render unreplicated replies.

* Add CancelKX and CancelMI commands.

## Bug fixes

* Fix paste image functionality.

# brclient

## Bug fixes

* Fix shadowed GC name check incase of searching for similar GC names.

* Fix hanging on compacting DB state.

* Reduce frequency of RMQ length check.

* Disable autocompact by default.

* Fix GC list command.

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu
- vcct94

## Changelog

All commits since the last release may be viewed on GitHub [here](https://github.com/companyzero/bisonrelay/compare/v0.2.3...v0.2.4).

