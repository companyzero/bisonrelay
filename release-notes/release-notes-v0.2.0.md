# bisonrelay-v0.2.0

See bisonrelay-v0.2.0-manifest.txt and the other manifest files for SHA-256 hashes and the associated .asc signature files to confirm those hashes.

See [README.md](./README.md#verifying-binaries) for more info on verifying the files.


# brclient

brclient has been given a lot of improvements for general usage and user
experience for 0.2.0.  There has been a revamp of how exchange rates are
consistently polled so that information can be available more often for the
various subsystem that require it.  

There was a bump of the underlying dcrlnd to v0.6.0.  This involved over 
590 commits across more than 645 files.  Those changes can be reviewed [here](https://github.com/decred/dcrlnd/compare/v0.5.0...v0.6.0). Overall, this will
lead to improved stability and efficiency with all the core LN infrastructure
that BR utilizes.


## Improvements

- Add spinner/loading animation to fetched pages loading.

- Add seed confirmation to all new wallet/BR set up.

- Add warning about invite keys is now shown to users about how to handle
  their invite keys.

- Make invite fetch async.

- Show last completed KX time of all counterparties.

- Add autosubscribe to posts whenever a user is KX'd with a novel user.

## Bug fixes

- Fix unable to subscribe to prepaid RV.

- Fix issue with reset causing autounsub with idle clients.

# Bison Relay GUI (bruig)

Bison Relay GUI was given an almost full design change for mobile and most of
the rough edges have been smoothed for the android implemenation. Due to 
the development teams reluctance to implement Firebase Cloud Messaging (FCM),
there was siginificant work done to limit the CPU usage on android.  There
should be further improvements in the future as we figure out other ways to 
utilize less overall CPU footprint. 

Notifications are now available on most platforms besides Windows.

Avatars are now supported for GUI.  Users may select an image to be used as
their avatar and that image will be automatically sent to other users that have
KX'd with them and will update in their clients automatically.

## Improvements

- Chat scroll has been improved and the position is now saved when clicked away.
  There is no longer a scroll that occurs when first going to a particular chat 
  window.

- Improve dropdowns overall styling and experience.

- Add confirmation dialogs for most major decisions to attempt to limit mistaken
  clicks.

- Refine chat lists into one long list and remove redundant buttons that
  accomplish the same functionality.

- Add a light theme that can be toggled in Settings.

- Add new functionality for creating a 'new message' and creating a new group
  chat.

- Replace multiple uses of reused StartupScreen to streamline widgets.

- Disable some buttons if client not connected.  Also make it much more clear
  when the client is not currently connected.

- Improve attachment UX and UI to follow a similar method that most other chat/
  social media applications use.

- Update GC message styling to also follow how other chat applications look and
  feel.

- Allow rendering PDFs in chats.  This may be replaced with just an option for 
  downloading, but for now they can be shown to the user in place.

- Add collapsable logs to startup screens to hide information that most users
  aren't required to view.

- Add timed profiling and zip logs for export to help developers track CPU usage
  and other platform specific information.

## Bug Fixes

- Properly scroll position.

- Scroll to last read now properly reacting to user interaction.

### Code Contributors (alphabetical order):

- Alex Yocom-Piatt (ay-p)
- David Hill (dhill)
- miki-totefu

## Changelog

All commits since the last release may be viewed on GitHub
[here](https://github.com/companyzero/bisonrelay/compare/v0.1.10...v0.2.0).
