# bisonrelay-v0.2.2

See bisonrelay-v0.2.2-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.


# Bison Relay GUI (bruig)

This release includes a full redo of material theming for GUI.  This will make 
all future changes or additions to themes extremely easy and ensures that no
unintended inconsistencies throughout.  

We have now also included a QR code creator (and scanner for the mobile version)
that should improve the way in which user invitations can be shared.  The 
overall generate/accept invite UI has also been improved and streamlined.

## Improvements

* New functionality to save and share pdf files from chat windows.

* New functionality to list all downloads and also have the ability to Cancel 
  selected downloads.

* New functionality to list all current invites.

* Posts now show their true creation timestamps, instead of the time in which
  they were fetched by the client.


## Bug fixes

* Fix syncing progress bar to properly show percentage complete.

* Show password errors on unlock screen.

* Fix text attachment rendering.

* Fix shutdown flow so restart after changing settings works properly on
  Windows.

# brclient

This release of brclient includes various minor bug fixes and includes
new commands for listing invites and downloads.

## Bug fixes

* Do not open chat window when avatar is updated.

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu

## Changelog

All commits since the last release may be viewed on GitHub
[here](https://github.com/companyzero/bisonrelay/compare/v0.2.1...v0.2.2).
