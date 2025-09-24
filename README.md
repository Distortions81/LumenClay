# aiMud

aiMud is a tiny, ANSI-friendly MUD server written in Go. This README covers how to build and run the server, connect as a player, and extend the built-in world data.

## Prerequisites

- **Go 1.25 or newer.** Earlier releases may not understand the module settings used by this project.
- **A telnet-capable client.** The examples below use the standard `telnet` utility, but any terminal program that can connect to TCP port 4000 and display ANSI colors (e.g., `nc`, SyncTERM, or a modern MUD client) will work.

## Building

Fetch dependencies and compile the server with Go:

```bash
go build ./...
```

This produces an `aiMud` binary in the repository root. You can also run the server directly without creating a binary by using `go run .` (see below).

## Running the server

Start the MUD server from the repository root:

```bash
go run .
```

The server listens on TCP port `4000` by default and prints a banner similar to:

```
MUD listening on :4000 (telnet + ANSI ready)
```

Leave this process running while clients connect.

To listen on a different host or port, supply the `-addr` flag. For example, to restrict the server to localhost on port 5000:

```bash
go run . -addr "127.0.0.1:5000"
```

Enable TLS by passing `-tls`. By default the server stores its certificate at `data/tls/cert.pem` and private key at `data/tls/key.pem`,
and you can override these paths with the `-cert` and `-key` flags:

```bash
go run . -tls -cert /path/to/cert.pem -key /path/to/key.pem
```

When the specified certificate or key files do not exist, `ListenAndServeTLS` automatically generates a self-signed certificate the
first time the server starts and reuses it on subsequent runs.

To stop the server, press `Ctrl+C` in the terminal running `go run .` or terminate the compiled binary if you used `go build`.

## Connecting via telnet

Open a second terminal and connect with telnet:

```bash
telnet localhost 4000
```

If the `telnet` command is unavailable on your platform, you can use `nc localhost 4000` or point a desktop MUD client to `localhost` port `4000` instead. The server uses ANSI color codes, so enable ANSI/VT100 interpretation in your client if it is optional.

## Accounts and authentication

- When you connect, the server prompts for a username. Entering a new name automatically starts account creation.
- You will be asked to supply a password of at least six characters. Passwords are stored hashed in `data/accounts.json`.
- Logging in with the username `admin` grants administrator privileges after the password is set, allowing access to administrative commands such as `reboot`.
- You have up to five attempts to choose a valid username and three tries per login to enter the correct password before the connection is closed.

## Basic commands for new players

After logging in, type `help` (or `?`) to see the in-game reference. Common commands include:

- `look` (`l`) &mdash; Re-describe your current room.
- `go <direction>` or `n`, `s`, `e`, `w`, `u`, `d` &mdash; Move between rooms.
- `say <message>` &mdash; Speak to everyone in your room.
- `whisper <message>` &mdash; Speak quietly; nearby rooms hear a muffled version.
- `yell <message>` &mdash; Broadcast to all connected players.
- `ooc <message>` &mdash; Out-of-character global chat.
- `emote <action>` or `:<action>` &mdash; Describe an action to the room.
- `who` &mdash; List connected players.
- `name <newname>` &mdash; Change your display name.
- `channel <name> <on|off>` / `channels` &mdash; Manage which chat channels you receive.
- `quit` &mdash; Disconnect from the server.
- `reboot` (admin only) &mdash; Reload the world data and return everyone to the starting room.

## Extending the world data

World rooms and areas are defined in JSON files stored under [`data/areas/`](data/areas/). Each file contains an object with a descriptive `name` and a list of `rooms`. Every room entry must provide:

- `id` &mdash; A unique string identifier used by exits and for spawning players.
- `title` &mdash; A short room name shown in the room header.
- `description` &mdash; Flavor text displayed when players enter or `look`.
- `exits` &mdash; A map of direction keywords (e.g., `n`, `south`, `up`) to destination room IDs.

To add new content:

1. Copy one of the existing area files (such as [`data/areas/garden.json`](data/areas/garden.json)) and update the `rooms` array with your new locations, descriptions, and exits.
2. Ensure that every exit target refers to a valid room ID. Exits can cross between files, so you can link different areas together.
3. Keep the JSON syntactically valid; `go fmt` can help format it, or use a JSON validator.
4. Rebuild or restart the server after saving your changes. Because the area files are embedded at compile time, live servers must be restarted (or admins can run `reboot` after recompiling) to load new room data.

With these steps you can grow the world organically while keeping the server lightweight and easy to run.
