# gh chat

_being a joke made as part of the GitHub Client Apps hackathon_

somewhere on a list of "terrible ideas for the github cli" is "realtime chat." naturally adding that as a joke has always amused me, so i finally did it.

<img width="911" alt="Screenshot 2022-12-02 120814" src="https://user-images.githubusercontent.com/98482/205377404-20db202d-c9d8-4ec9-a108-d7ec26d24ee3.png">

## features

- nick list
- invites with push notification
- join/part messages
- visual bell on @ mention
- ephemeral. deletes all messages on host exit.
- figlet support
- emoting
- SMTP gateway (assuming you haven't muted email gist notifications)
- 1337

## install

`gh ext install vilmibm/gh-chat`

## usage - create a chatroom

prior to launching the new chatroom, the room ID will be printed for copy and pasting.

`gh chat`

## usage - join a chatroom

`gh chat <chatroom ID>`

a chatroom ID looks like `b6f867cbdd5dcb3e08fca1323fae4db8` and you might see one in your GitHub notifications.

## chat commands

- `/invite <user>` invite a user to join you. they'll get a GitHub notification.
- `/me <text>` do an emote
- `/banner <text>` render your `text` as an ascii banner
- `/banner-font <font> <text>` render `text` in the chosen `font`. try `script` or `shadow` for fonts.
- `/quit [<msg>]` quit chat with optional departure message. you won't see this, but others in the chat will.

## future direction

the extremely high tech backend for this (gist comments) has a fatal
flaw--though @ mentioning someone in a gist comment does generate a
notification that can be seen in web and email, gist notifications are filtered
out of the API response for notifications. this means I can't programmatically
check for chat invites and then interrupt someone's terminal to inform them
they have a chat invite. i think this is a bug in .com and i hope we fix it.

