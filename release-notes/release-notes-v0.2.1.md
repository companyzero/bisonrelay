# bisonrelay-v0.2.1

See bisonrelay-v0.2.1-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.


# Bison Relay GUI (bruig)

This release includes numerous minor bug fixes and some improved network
configuration capabilities.  

The Desktop version of the settings page has received an overhaul to make it
more intuitive and functional.

The underlying dcrlnd software has also been updated.

## Improvements

* User avatars are now seen everywhere (Feed, etc).  And they are now clickable 
  to show the submenu from everywhere. 

* Add network configuration screen to start up to allow for various alternative
  specialized networking setups.

* Refactor models to allow for more flexibility with Chats, Feed, and menus.

## Bug fixes

* Fix link display colors and message margins

* Make chat colors consistent across mobile and desktop versions.

* Fix newline when using ctrl-enter.

* Fix respecting CLI args with AppImage.

* Update subscribe/unsubscribe in the menus.

* Fix dropdown colors.

* Prevent flicker in the LN channels screen and make channel list scrollable.

* Fix chat scrolling and code block rendering.


# brclient

Pages are now available for viewing while the client is offline.  

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu

## Changelog

All commits since the last release may be viewed on GitHub
[here](https://github.com/companyzero/bisonrelay/compare/v0.2.0...v0.2.1).
