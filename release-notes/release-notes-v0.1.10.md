# bisonrelay-v0.1.10

See bisonrelay-v0.1.10-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.


# brclient

There has been a lot of added commands and improvements since the last release.

The most important improvements lie with upcoming server policy changes to 
allow for various updates to existing restrictions (eg. max message size of 
1mb).

There was also a bump of the underlying dcrlnd to v0.5.0.  This involved over 
1000 commits across more than 800 files.  Those changes can be reviewed [here](https://github.com/decred/dcrlnd/compare/v0.4.0...v0.5.0). Overall, this will
lead to improved stability and efficiency with all the core LN infrastructure
that BR utilizes.

## Improvements

- Add Rescanwallet command

- Add List On-Chain Transactions command

- Add receive receipts for posts and comments.

- Send and show post receive receipts.

- Replace Bittrex with MEXC for rates.  

- Move various invite commands under /invite.

- Add ModifyGCOwner and add commands to client and GUI.


## Bug fixes

- Add missing data to the FetchResource request
 
# Bison Relay GUI (bruig)

This marks the initial release of the mobile BR implementation.  The UX/UI is 
still a work in progress for a few areas, but generally should be easy to use.

We've focused on having the BR mobile application be as easy to understand as 
possible, since this will be aimed at less technical users.

Currently, we're just supporting Android apks, which can be installed when
users allow for 3rd party installations in the developer settings.

## Improvements

- Refine UX/UI for mobile usage.

- Refine font size across all widgets and screens.  There are now 3 sizes
  that can be used throughout and those sizes are centralized in one location
  to have a consistent look and feel throughout.

- Add Reset All KX button.  Even though this is done automatically on a periodic
  basis, there is now a button that forces BR to KX with everyone.  This should
  un-wedge clients in strange situations.

- Add Subscribe button to feed comments.  This should allow users follow others
  more easily.

- Add Transitive Reset button.

- Add Un-optimize Battery for mobile.  Due to not using Firebase Cloud Messaging
  (FCM), we noticed having unrestricted battery for BR allows for better message
  retrieval.

## Bug Fixes

- Fix jumping to beginning of day message in GCs

- Add newly created GCs to current chat list.

- Filter historical logs based on rules.

- Allow posts and feed to be selectable.

- Fix Generate Invite on mobile.

- Remove GC avatar long press on mobile.

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu
- vctt94

## Changelog

All commits since the last release may be viewed on GitHub
[here](https://github.com/companyzero/bisonrelay/compare/v0.1.9...v0.1.10).
