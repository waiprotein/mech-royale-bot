# Mech Royale bot

Our entry for **Builders – Mech Royale** (joinbuilders.org buildathon): a bot that pilots one mech in the 16-mech `mechfight/1` battle royale over JSON lines on stdin/stdout. Go, standard library only.

## Code we brought (credit)

This bot is based on **giddygiddyboy's public Mech Royale bot**, used under its MIT licence:

- Source: https://github.com/justnau/giddygiddyboy-mech-royale (imported at commit `8b67d5b`, 2026-11-21; `mech/brain.go` updated to upstream `e19e3c8`, taken from the MIT-licensed copy vendored in https://github.com/charliescull/mech-royale-bot under `drivers/giddy/`)
- Licence: MIT, copyright (c) 2026 giddygiddyboy, kept in `LICENSE`
- Our changes are listed in the git history after the import commit.

The protocol itself comes from the organiser's `mechfight-kit` (`protocol.md`).

## Our original work (on top of giddy's base)

In numbers (Sun 22 Nov, 12:40 GMT): `mech/brain.go` is **4,558 lines, of which about 2,600 are ours** (giddy's file had 2,001; we removed 38 of its lines). It has **95 top-level functions, 54 of them new** (giddy's had 41). On top of that we wrote **29 unit-test files (~3,000 lines; giddy ships no tests)**, the WebAssembly test build (`cmd/wasmbot`) and an RNG salt (`mech/salt.go`), over 70 commits after the import. `mech/world.go` (map, line of sight, pathing) and `mech/types.go` are giddy's, essentially unchanged. About 40% of the shipped Go code is ours.

Every addition below is our own code, behind a parameter in `Params` (documented next to each field), and the submitted build sets its parameters in `DefaultParams()`. All in `mech/brain.go`; see the git history for every step.

- **Free-for-all detection and strict gating (`FFAStrict`, `FFAStrictN`).** Our FFA behaviours switch on only after the bot has proven it is in a free-for-all (one or two deaths it did not cause), so the practice house squad sees a different policy from the FFA.
- **Our FFA brain (validated in our own 16-mech simulations, both against giddy-majority fields and against fields with rival bots):** gun-only vs armed foes counts as outgunned (`GunOnlyOut`, `GunOnlyR`); lightning-only breaks line of sight from laser holders (`LightLOS`); long-range laser openers on mechs that are unaware or busy, fought from range (`LRLaser`, `LRHP`, `LRKeep`); hide-and-regenerate latch (`RegenHide`); home spots out of sight of fresh mechs (`HomeLOS`); watching the last foe when idle (`WatchLast`); breaking the line of a laser charge aimed at us (`LaserLOS`, `laserThreats`, `laserCover`); end-game retreat and hold-out away from the final pile-up (`LateR`, `HoldOut`, `HoldMin`, `holdHome`); a fight-or-flee duel model fitted on recorded showdown engagements (`duelOdds`, exchange tracker).
- **Opening:** arm early with a second weapon (`Arm`, `ArmFirst`, `ArmBoost`, `Scan`, `ArmStrict`, `ArmUntil`), and a calm 360° rescan (`Rescan`).
- **Practice-mode (house-squad) play:** squad laser range (`SqLaserRange`), late cautious play (`CautiousT`), rim holding (`PracRim`, `SqRim`), back-turned openers and squad long-range laser (`SqBackOpen`, `SqLRLaser`), regen-window target lock (`RegenLock`), close hunting (`SqHunt`), squad retreat/return and turning on a lone chaser (`SqRet`, `SqRetUp`, `SqCorner`), practice / pre-latch engage radius (`PEngage`), and more levers (rear-arc avoidance, cover while reloading, laser-cooldown punish, lightning stand-off, kill-aware fire control, kiting, cornered cover, speed/dodge, drop capture).
- **Safety:** a crash breaker (`safeFallback`): after 3 recovered panics in a match every one of our additions switches off and the bot plays plain giddy; the first panics are logged to stderr.
- **Tooling (in this repo):** `MECH_FLAGS` parameter overrides for local tests (unset in matches), `cmd/wasmbot` for our local simulator, and the unit tests in `mech/*_test.go`. Our simulator, league runner and analysis tools live in a separate private lab repo.

## Layout

| Path | What it is |
| --- | --- |
| `Dockerfile` | two-stage build: pinned `golang:1.26.1-bookworm` (by digest) builds a static linux/amd64 binary, which runs from `scratch` |
| `go.mod` | Go module `mechbot`, no dependencies |
| `cmd/bot/main.go` | stdin/stdout loop: answers only the newest observation, echoes the tick |
| `mech/` | world model (`world.go`), decision making (`brain.go`), protocol types (`types.go`) |

## Build and check locally

From `mech_buildathon/` (the kit sits in `kit/mechfight-kit/`):

```sh
cd bot && go build -trimpath -o mechbot.exe ./cmd/bot && cd ..
cd kit/mechfight-kit && python check/check.py -- ../../bot/mechbot.exe
```

With Docker (the match sandbox):

```sh
docker build --platform linux/amd64 -t mech-royale-bot bot
python3 kit/mechfight-kit/check/check.py --map-dir /arena -- docker run --rm -i --network none --cpus 1 --memory 512m --read-only --tmpfs /tmp:rw,size=64m --user 65534:65534 -v "$PWD/kit/mechfight-kit/arena:/arena:ro" mech-royale-bot
```

## Submitting

Push to the public GitHub repo (Dockerfile at the top, well under 20 MB) and submit its URL on the Mech Royale track page. Keep the repo public until results are published.

## Licence

MIT, see `LICENSE`.
