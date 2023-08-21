# bisonrelay-v0.1.8

This is another large release for Bison Relay that covers numerous improvements,
bug fixes and new features.  

Chat history is now loaded when opening a chat window in brclient or GUI.
Currently, we're just loading the last 500 lines from the logs.  Depending on
user feedback we can look into allowing users to change that when loading new
chat windows.

We've also made large strides in getting mobile builds working.  The backend
code has been updated to work with Go Mobile which will allow for the dcrlnd and
client code to work on mobile platforms.  The GUI has been updated to provide a baseline responsiveness for small screens.  We are currently implementing a new
design from @saender to have a mobile specific UX/UI.

Simplestore/pages ?

# brclient

## Features 

* 

## Improvements

* 

## Bug Fixes

*

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

