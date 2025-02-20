# bisonrelay-v0.2.3

See bisonrelay-v0.2.3-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.

## Contents
* [Bison Relay GUI](#bruig-v023)
* [brclient](#brclient-v023)


# Bison Relay GUI (bruig)

This release includes various bug fixes and general improvements to the Bison Relay Desktop GUI.  Emoji selection has been added, as well as image compression options.

We've also bumped the max chunk size to 10Mb which should allow for better file transmission moving forward.

## Improvements

* Add image compression feature.
* Show Un-kxed group chat members (also show warning about this state).
* Show list of active KX attempts.
* Add audio notes and other audio related internal packages that will be used for future features.
* Add AVIF image support.
* Allow for images to be pasted into chat input field.
* Improve Feed UI/UX.

## Bug fixes

* Improve blockquote colors.
* Fix issues with added CLRF from windows that caused excessive blank lines.
* Fix toggle of meta keys which should fix various key press issues while entering input text (in Feed or Chat).
* Fix paste shortcut.


# brclient

Bison Relay for CLI has also received the Max Chunk size improvement.  Audio notes have been added along with an 'editor' feature.

## Bug fixes

* Ensure users that were muted no longer are shown in GCs even after alias change.
* Fix embedded viewer usage so applications used for image viewing don't spawn from brclient (and thereby taking focus/control away from brclient).

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu

## Changelog

All commits since the last release may be viewed on GitHub [here](https://github.com/companyzero/bisonrelay/compare/v0.2.2...v0.2.3).

