# Mech Royale bot

Our entry for **Builders – Mech Royale** (joinbuilders.org buildathon): a bot that pilots one mech in the 16-mech `mechfight/1` battle royale over JSON lines on stdin/stdout. Go, standard library only.

## Code we brought (credit)

This bot is based on **giddygiddyboy's public Mech Royale bot**, used under its MIT licence:

- Source: https://github.com/justnau/giddygiddyboy-mech-royale (imported at commit `8b67d5b`, 2026-11-21; `mech/brain.go` updated to upstream `e19e3c8`, taken from the MIT-licensed copy vendored in https://github.com/charliescull/mech-royale-bot under `drivers/giddy/`)
- Licence: MIT, copyright (c) 2026 giddygiddyboy, kept in `LICENSE`
- Our changes are listed in the git history after the import commit.

The protocol itself comes from the organiser's `mechfight-kit` (`protocol.md`).

## What we changed (our own work, on top of giddy's base)

All in `mech/brain.go` unless noted; see the git history for every step.

- **Free-for-all detection and strict gating (`FFAStrict`).** Our FFA behaviours switch on only after the bot has proven it is in a free-for-all (a death it did not cause), so they never act against the practice house squad.
- **End game:** retreat stays allowed until the next circle is 25 m (`LateR`), and the bot holds a hidden spot away from the final-centre pile-up until the zone forces it in (`HoldOut`, `HoldMin`, `holdHome`).
- **Duel model (`duelOdds`, `DuelFlee`):** fight-or-flee decisions from a logistic model we fitted on recorded top-16 showdown engagements (first shot, HP ratio, lasers, facing).
- **Weapon and engagement rules:** gun-only versus an armed mech counts as outgunned (`GunOnlyOut`); lightning-only breaks line of sight from laser holders beyond lightning range (`LightLOS`); facing unexplained weapon sounds (`FaceSound`); third-party joins from farther than the target's current opponent (`TPJoin`); rear-arc openers (`BackOpen`); long-range laser openers on unaware or busy mechs with less HP, fought from long range (`LRLaser`, `LRHP`, `LRKeep`); laser pre-charge discipline (`PreCharge`, `PeekCD`); spacing (`CrowdR`, `HomeFrac`); the alive-count part of giddy's no-retreat end game (`LateAlive`).
- **Experiment plumbing:** `MECH_FLAGS` parameter overrides for local tests (unset in matches), a `Salt` for RNG re-rolls, a beam-based FFA detector (`BeamFFA`, off), and a WebAssembly test build (`cmd/wasmbot`, not part of the image).
- Which of these are switched on in the submitted build is set in `DefaultParams()`.

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
