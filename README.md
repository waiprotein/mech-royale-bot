# Mech Royale bot

Our entry for **Builders – Mech Royale** (joinbuilders.org buildathon): a bot that pilots one mech in the 16-mech `mechfight/1` battle royale. It reads JSON lines on stdin and writes one action per observation on stdout. The rules are in the organiser's `protocol.md` (kit).

| File | What it is |
| --- | --- |
| `Dockerfile` | the submission image (`python:3.13.18-slim`, runs `python3 -u /bot/bot.py`) |
| `bot.py` | our entry point: `main()` with a hardened stdin/stdout loop and a safety net; decision code in `Bot.act()` |
| `mechfight.py` | the kit's wrapper (unchanged): typed messages, debug drawing, `log()` |
| `example_bot.py` | the kit's example bot (unchanged), kept as the baseline reference |

`bot.py` currently makes exactly the same decisions as `example_bot.py`. If decision code raises, the error goes to stderr and the bot still answers that tick (echoing it) with its last good `move`/`aim`/`boost`, so the mech coasts instead of dying.

## Conformance check (local)

The organiser's `check/check.py` and `stream.jsonl.gz` are **not** in this repo: they live in the kit next to it, and a wrapper lives in `../tools/`. This keeps the repo (and the Docker build context) to what the bot needs at runtime, far under the 20 MB limit. Layout assumed:

```
mech_buildathon/
  bot/                 <- this repo (Dockerfile at its top)
  kit/mechfight-kit/   <- official kit, untouched (protocol.md, check/, arena/)
  tools/check.sh       <- runs the kit's check.py against bot/bot.py
```

From `mech_buildathon/` (Git Bash on Windows, or any POSIX shell):

```sh
bash tools/check.sh              # python -u bot/bot.py, no sandbox
bash tools/check.sh --docker     # build this repo and check it with the match sandbox flags
```

Or by hand, from `kit/mechfight-kit/`:

```sh
python3 check/check.py -- python3 -u ../../bot/bot.py
docker build -t my-bot ../../bot
python3 check/check.py --map-dir /arena -- docker run --rm -i --network none --cpus 1 --memory 512m \
  --read-only --tmpfs /tmp:rw,size=64m --user 65534:65534 -v "$PWD/arena:/arena:ro" my-bot
```

The sandboxed run (no network, 1 CPU, 512 MB, read-only disk except `/tmp`, uid 65534) is what the scorer runs before matches; a bot that passes only without those flags is rejected.

## Submitting

1. Commit with `bash tools/check.sh` passing; one behavioural change per submission.
2. Push to the **public** GitHub repo (Dockerfile at the top level; ≤ 20 MB at the submitted commit; built on linux/amd64 in ≤ 10 min).
3. Submit the repo address on the Mech Royale track page. Each submission plays 48 practice matches against built-in bots. At most 4 submissions per hour.
4. Log it in the vault's `Submissions/Submission log.md`. Keep the repo public through the results.

## Runtime rules (do not break)

- stdout carries protocol lines only; logs go to stderr.
- Answer every observation within 50 ms, echoing its `tick`; answer only the newest when several are queued.
- Write files only under `/tmp`; install any package system-wide in the Dockerfile with a pinned version (no network at runtime).
- Read every game number from `hello["config"]`; read the map from `hello["map"]["dir"]`.
