# Polyarena

A 2D multiplayer tank arena in the diep.io genre: shoot polygon shapes for XP,
level up, spend points on stats, branch into stronger tank classes, and fight
other players. TypeScript + HTML canvas client, Go WebSocket server.

## Run

```sh
# client bundle (once, or `npm run watch` while developing)
cd client && npm install && npm run build

# server (serves the client and the websocket on :8080)
cd ../server && go run .
```

Open http://localhost:8080. `./run.sh` does both.

Server flags: `-addr :8080`, `-static ../client/public`, `-mode ffa|teams`,
`-teams 2` (2–4 teams in teams mode). For example `./run.sh -mode teams -teams 3`.

## Controls

| Key | Action |
| --- | --- |
| WASD / arrows | move |
| mouse | aim, left button fires |
| E | toggle auto-fire |
| 1–8 | upgrade the matching stat |
| Y / U / I / O | pick a class upgrade when one is offered |

## Gameplay

- **Shapes** — squares (10 xp), triangles (25 xp), pentagons (130 xp). The
  centre of the map is a pentagon nest with a few huge alpha pentagons
  (3000 xp).
- **Levels** — 45 levels. Every level up to 28 grants a stat point, then one
  every third level (33 total). 8 stats, 7 points max each.
- **Classes** — Scout → (lvl 15) Twin, Marksman, Spray, Flanker → (lvl 30)
  Trident, Cross, Twin Flank, Railgun, Cannon, Hailstorm, Thruster → (lvl 45)
  Fan, Star, Longbow, Siege. Definitions live in `server/classes.go`;
  barrel geometry is sent to the client so it renders whatever the server
  defines.
- **PvP** — the server runs one of two modes:
  - **Free-for-all** (`-mode ffa`, the default): every tank is hostile. Your
    tank is blue, everyone else is red.
  - **Teams** (`-mode teams -teams N`): players are placed on the smallest of
    Blue, Red, Green and Purple when they join and keep that team across
    respawns. Teammates cannot damage each other, allied bullets pass through
    allied tanks and bullets, and each team spawns in its own corner of the
    map. The leaderboard shows team totals above the top players, and your
    own tank gets a white centre dot so you can tell it from teammates.

  Killing a tank grants a third of its score. Fresh spawns have a 4 s shield
  that drops when they fire.

## Architecture

```
server/
  main.go     http + static files + /ws
  net.go      websocket client: read/write pumps, message dispatch
  game.go     world simulation (60 Hz), collisions, XP/levels, snapshots (20 Hz)
  classes.go  tank class tree and barrel definitions
  teams.go    game modes: team assignment, team spawns, friendly-fire rules
client/src/
  main.ts     input, camera, game loop
  net.ts      websocket wrapper
  interp.ts   snapshot buffer + interpolation (110 ms behind the server)
  render.ts   canvas rendering: grid, shapes, bullets, tanks, minimap
  ui.ts       HTML HUD: stats, class picker, leaderboard, xp bar, overlays
  types.ts    protocol types
```

Protocol: little-endian binary websocket frames, each starting with a one-byte
type tag. The full layout is documented at the top of `server/protocol.go`;
`client/src/protocol.ts` is its mirror and the two must change together.

- client → server: `Join`, `Input` (6 bytes), `Stat`, `Upgrade`
- server → client: `Init` (world size, mode and team names, stat names, class
  tree), `State` (snapshot), `Dead`

The server is authoritative: clients only send intent. Each client receives
only the entities inside its own (FOV-scaled) view rectangle. Positions are
u16 fixed-point (0.1 px), angles u16, HP a u8 fraction, and "is this mine"
a flag, so a typical entity costs 12–16 bytes. Tank names are sent the first
time a client sees that tank and the leaderboard only when it changes; both
are marked delivered only if the frame was actually queued, so a frame dropped
for a slow client doesn't lose them.
