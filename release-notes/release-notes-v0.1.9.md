# bisonrelay-v0.1.9

See bisonrelay-v0.1.9-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.

This is a minor update to brclient and GUI that mostly fixes some bugs that
were reported in v0.1.8.  Simplestore issues that were reported have been fixed.
We've also included a major push to implement the mobile design from @saender.
  

# brclient

## Features

* Automatic unsubscrive and GC kick for idle users.  

## Improvements

* Install simplestore templates if store directory is empty.

* Update bubbletea chat dependencies to hopefully fix various issues with
  chat.

* Autohandshake with idle users.

* Expose FirstCreated ad LastHandshakeAttempt for use in various spots.

## Bug fixes

* Various simplestore and local pages fixes.

# Bison Relay GUI (bruig)

## Improvements

* Show already openned DM members in Address Book as well.  Now all users,
  whether currently chatting with them or not will be seen in the Address
  Book.

* More mobile design improvements based on design delivered by @saender.

* Display UserPosts the same as the posts list, instead of just text in the DM 
  chat windows.

* Add reset all old KX button to Settings

## Bug Fixes

* Fix link openning in chat messages and posts.

* Fix channel creation lock out when starting with no channels.

* Fix unread comments in posts indicator.  


### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- Dave Collins (davecgh)
- David Hill (dhill)
- miki-totefu

## Changelog

All commits since the last release may be viewed on GitHub
[here](https://github.com/companyzeron/bisonrelay/compare/v0.1.8...v0.1.9).
