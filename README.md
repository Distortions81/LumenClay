# LumenClay

LumenClay is a tiny, ANSI-friendly MUD server written in Go. This README covers how to build and run the server, connect as a player, and extend the built-in world data.

## Prerequisites

- **Go 1.25 or newer.** Earlier releases may not understand the module settings used by this project.
- **A telnet-capable client.** The examples below use the standard `telnet` utility, but any terminal program that can connect to TCP port 4000 and display ANSI colors (e.g., `nc`, SyncTERM, or a modern MUD client) will work.

## Building

Fetch dependencies and compile the server with Go:

```bash
go build ./...
```

This produces an `LumenClay` binary in the repository root. You can also run the server directly without creating a binary by using `go run .` (see below).

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

Player data defaults to the files under [`data/`](data/). Use `-accounts` to choose a different account database and `-areas` to load world definitions from another directory:

```bash
go run . -accounts /var/lumen/accounts.json -areas /opt/world-data
```

When overriding the accounts file, persistent mail and offline tells automatically live beside it (for example `/var/lumen/mail.json` and `/var/lumen/tells.json`). You can point each of these stores elsewhere with `-mail` and `-tells` if desired:

```bash
go run . -accounts /var/lumen/accounts.json -mail /srv/mailbox.json -tells /srv/tells.json
```

Enable TLS by passing `-tls`. By default the server looks for certificate files in the project root that follow the
[Certbot](https://certbot.eff.org/) naming convention: `fullchain.pem` and `privkey.pem`.
The MUD listener and the staff web portal share these files so a single certificate
covers both telnet and HTTPS. Point `-cert` at another directory or bundle if your
files live elsewhere. When you supply a directory, the server reads `fullchain.pem`
and `privkey.pem` from within it. When you supply an explicit certificate file,
`privkey.pem` in the same directory (or a `.key` that shares the certificate's base
name) is used for the private key:

```bash
go run . -tls -cert /etc/letsencrypt/live/example.com
```

When HTTPS is enabled, the staff web portal automatically binds to the same host as the MUD listener on port `443` and reuses the
TLS certificate and key. Override the default port or certificate bundle if you need to split the services:

- `-web-addr auto` (default) &mdash; use the host from `-addr` on port `443`.
- `-web-addr off` &mdash; disable the portal entirely.
- `-web-addr PORT` &mdash; listen on a custom HTTPS port while keeping the host from `-addr`.
- `-web-cert auto` &mdash; reuse the `-cert` path (default).
- `-web-cert PATH` &mdash; point the portal to a different certificate directory or bundle.
- `-web-base-url https://staff.example.com` &mdash; publish the portal through an external HTTPS origin (for example, a reverse
  proxy). The server appends `/portal/<token>` to this base when it issues one-use links.

The server generates a self-signed certificate the first time it starts if the specified
files do not exist and reuses that certificate afterwards.

### Staff web portal

When HTTPS is enabled (see `-tls` above) staff can request a one-use link with the
`portal` command to reach a browser dashboard. The portal now includes:

- Real-time "At a Glance" cards that summarize total online players, staff coverage, and average session length.
- A detailed player table with level, health, mana, connected-room information, and live session timers.
- JSON APIs at `/api/players` (player list + stats) and `/api/overview` (aggregated staff metrics) for custom tooling.

Choose which account should receive administrator privileges by using the `-admin` flag (case-insensitive). For example, to grant the
`Wizard` account admin rights:

```bash
go run . -admin Wizard
```

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
- Logging in with the username specified by the `-admin` flag (default `admin`) grants administrator privileges after the password is set, allowing access to administrative commands such as `reboot`.
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
- `buildhelp` (builders/admins) &mdash; List the online creation commands available to builders.
- `portal [builder|moderator|admin]` (builders/moderators/admins) &mdash; Generate a one-use HTTPS link to the staff web portal when it is configured.
- `wizhelp` (admin only) &mdash; List administrative commands such as `reboot` and `summon`.

Climb to the Glazemaker's Overlook from the starting atrium and head north to reach the new Celestial Observatory. There you'll find the Horizon Plaza, Zephyr Rampart, Astral Scriptorium, and the Lenswright Workshop, now joined by the Arcade of Shifting Sundials, a noctilucent reflecting pool, and an expanded vertical circuit that threads through the Aurora Spire, its heliograph gallery, a chart vault walkway, and the tea-scented loft of Professor Orrin before cresting at the beaconry. The subterranean Starwell, Resonance Vault, and Gravity Underchamber remain below, rounding out a sky-struck ascent packed with NPCs and artifacts.

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

## World scripting

Areas, rooms, NPCs, and items can all run lightweight Go scripts interpreted by
[Yaegi](https://github.com/traefik/yaegi). Each object stores its script directly in
the world JSON under a `"script"` field containing a small `package main` snippet.
Scripts are compiled the first time the server needs them and cached for later reuse.

### NPC hooks

NPC scripts can expose:

- `func OnEnter(ctx map[string]any)` whenever a player enters the room.
- `func OnHear(ctx map[string]any)` after a player uses the `say` command in that room.

The context map provides:

| Key          | Type           | Description |
|--------------|----------------|-------------|
| `"say"`      | `func(string)` | Broadcast a line to the room as the NPC. |
| `"emote"`    | `func(string)` | Perform an emote-style action. |
| `"tell"`     | `func(string)` | Whisper privately to the speaking player, if any. |
| `"speaker"`  | `string`       | Name of the triggering player, or an empty string. |
| `"message"`  | `string`       | Raw text the player spoke (only set for `OnHear`). |
| `"npc"`      | `string`       | NPC name. |
| `"room"`     | `string`       | Room identifier. |

### Room hooks

Rooms may define:

- `func OnEnter(ctx map[string]any)` after the description is shown to an entering player.
- `func OnLook(ctx map[string]any)` whenever someone `look`s without a target.

Room contexts include:

| Key          | Type           | Description |
|--------------|----------------|-------------|
| `"narrate"`  | `func(string)` | Send italicized narration to the triggering player. |
| `"broadcast"`| `func(string)` | Send an atmospheric line to everyone in the room. |
| `"room"`     | `string`       | Room identifier. |
| `"player"`   | `string`       | Player name, if available. |
| `"via"`      | `string`       | Direction or method the player used to arrive. |
| `"hook"`     | `string`       | Name of the hook that fired (e.g. `OnEnter`). |

### Area hooks

Area scripts run when a player enters any room sourced from that area file.
They support `func OnEnter(ctx map[string]any)` with access to:

| Key          | Type           | Description |
|--------------|----------------|-------------|
| `"narrate"`  | `func(string)` | Send a private ambient line to the player. |
| `"broadcast"`| `func(string)` | Share a message with the whole room. |
| `"area"`     | `string`       | Area display name. |
| `"room"`     | `string`       | Room identifier. |
| `"player"`   | `string`       | Player name, if available. |
| `"via"`      | `string`       | Direction or method the player used to arrive. |

### Item hooks

Items can implement `func OnInspect(ctx map[string]any)`, which runs when a player
examines an item in the room or in their inventory. The context offers:

| Key          | Type           | Description |
|--------------|----------------|-------------|
| `"describe"` | `func(string)` | Append additional flavor text for the inspecting player. |
| `"item"`     | `string`       | Item name. |
| `"room"`     | `string`       | Room identifier, if applicable. |
| `"where"`    | `string`       | Location hint (`"room"` or `"inventory"`). |
| `"player"`   | `string`       | Player name, if available. |

Scripts can freely import Go standard library packages (such as `strings`) and compose
these helpers to build rich behaviors without referencing internal engine code.
