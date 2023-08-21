# bisonrelay-v0.1.8

See bisonrelay-v1.8.0-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.


This is another large release for Bison Relay that covers numerous improvements,
bug fixes and new features.  

With the release of Bison Relay v0.1.8, support for digital-only ecommerce sites
has been added, along with pages, client-side filtering, and client backups.  
Since Bison Relay is a standalone alternative platform to the web that tightly
integrates money via the Lightning Network, it is natural to use it as a basis
for ecommerce sites.  The initial ecommerce infrastructure is called 
simplestore and supports selling digital-only goods, e.g. images, videos, 
audio, and files, where payments can be received both via on-chain payment in
Decred or via the Decred Lightning Network.  Documentation on how to setup a
simplestore site can be found [here](https://github.com/companyzero/bisonrelay/blob/master/doc/simplestore.md).

As part of this initial ecommerce release, the concept of pages was added, which
is similar to web pages.  In order to operate a basic ecommerce site, it is
necessary to have the site information organized into pages that reference and
link to each other. These sites, whether ecommerce or other collections of
pages, are hosted from an individual Bison Relay client, so the client also acts
as a server for sites and pages.

To date, social media services have implemented several forms of server-side
filtering, where server administrators determine what should and should not be
filtered and how to execute the filtering.  Bison Relay is all about making
users sovereign over their data and communications, and not only is server-side
filtering a violation of users’ sovereignty, it is not feasible to implement
when all messages are encrypted and there are no user accounts on the server.
To address this issue, we have added client-side filtering to allow users to
filter messages that they would prefer to not see, instead of having server
administrators centrally plan the filtering.

Backups are a major weak point of most cryptocurrency tools, particularly when
working with the Lightning Network.  Bison Relay now has a basic backup tool
that creates a backup of your contacts and your Lightning Network channels.
In the event a client has unrecoverable errors of any kind, it can be restored
using the initial client wallet seed and this basic backup information.  Any
contacts added after the most recent backup will need to be manually re-added
after restoring from the seed and the backup.

Chat history is now loaded when opening a chat window in brclient or GUI. 
Currently, we're just loading the last 500 lines from the logs.  Depending on
user feedback we can look into allowing users to change that when loading new
chat windows.

We've also made large strides in getting mobile builds working.  The backend
code has been updated to work with Go Mobile which will allow for the dcrlnd and
client code to work on mobile platforms.  The GUI has been updated to provide a baseline responsiveness for small screens.  We are currently implementing a new
design from @saender to have a mobile specific UX/UI.


# brclient

## Features
 * content filtering (/filters)
 * basic backup (/backup destdir)
 * simple store for selling digital files (see docs)

## Improvements
 * Add syncfreelist option in bbolt for improved startup time.  Defaults to true.
 * Force recheck on server after a channel changes status, speeding up reconnect.
 * Switch brclient to strict INI.
 * Attempt auto key exchange when sending groupchat messages when an unknown member exists
   in the groupchat.
 * Display newly kx'd users in groupchats
 * Remove old mediate ID requests using server expiry.
 * Display chat history in PM's and GC's
 * Create version 1 groupchats by default.
 * Confirm comment before submitting.
 * Allow using $EDITOR for post comments. (/post)
 * Allow closing channels with the channel prefix.
 * Add client 3*way handshake feature to test ratchets (/handshake)
 * Fetch rates from Bittrex as well as dcrdata.
 * Sort posts by recent activity
 * Add command to show running Tip attempts (/runningtips)
 * Pass proxy settings to DCRLND backend.
 * Support br:// links to other users pages.

## Bug fixes
 * Switch HTTP client idle connections from 100 to 2.
 * Fix duplicated messages in new chat windows.
 * fix concurrent autokx causing broken ratchets.
 * Fix possible panic in /ln channels.


# Bison Relay GUI (bruig)

## Features

* We've added an 'Address Book' feature.  Now chats will only be populated in
the chats list if there is some sort of history in the chat.  The user may 
also 'hide' the chat and remove it from the list if they'd like.  To start a
new chat, the user can click the button to show the address book and then 
they can click the start chat button.

## Improvements

* Notifications are now shown in the sidebar for both new chat and new posts
  or new comments.  These notifications are shown until the new content has
  been viewed.  

* Chat areas have been improved for scrolling and various other quirks with
  input entry.  Hopefully this should reduce strange scroll bounces and issues
  with inputing while actively receiving messages.

* As mentioned above, we've done a first pass for small screen/mobile
  responsiveness.  While we are implementing the final designs offered by 
  @saender, this first pass should allow basic usage while in a very small
  screen width.

* Feed is now being sorted by last activity.  Whatever is the most recent 
  (new comment or new post) will be shown at the top then in descending order
  below.

* The comment areas below all posts have been improved and refined to allow for
  more easy thread/comment tracking and conversations.  

* Chat lists have been improved to now be collapsable and new chat buttons
  in more streamlined locations.

* We are now able to show embedded pictures in the feed posts screen.  

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu
- Tiago Alves Dulce (tiagoalvesdulce)

## Changelog

All commits since the last release may be viewed on GitHub
[here](https://github.com/companyzeron/bisonrelay/compare/v0.1.7...v0.1.8).
