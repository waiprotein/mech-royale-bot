"""Mech Royale entry point (mechfight/1). Started by the Dockerfile as `python3 -u /bot/bot.py`.

Behaviour today: identical decisions to the kit's example_bot.py (the baseline, kept unchanged next
to this file): walk to the next zone centre, take a non-gun weapon if one is in reach, aim and fire
both slots at the nearest visible mech, jump every 40 ticks, boost when outside the zone.

What this file adds over example_bot.py + mf.run():
- a main() with its own hardened stdin/stdout loop (same rules as mf.run: `ready` on `hello`,
  answer only the newest queued observation, echo its tick, exit on `end` or EOF);
- a safety net: any exception in decision code is logged to stderr (rate-limited) and answered
  with a safe default action that echoes the tick (last good move/aim/boost, i.e. a coast);
- strict JSON out (NaN/inf in an action is treated as an error, not sent as invalid JSON);
- protocol lines are written as bytes with a bare "\n", logs go to stderr only.

Read protocol.md before changing behaviour. Rules: stdout is protocol only; never write outside
/tmp; answer within 50 ms; read every number from hello["config"].
"""
from __future__ import annotations

import json
import math
import queue
import sys
import threading
import traceback
from typing import Any, Dict, Optional

import mechfight as mf

BOT_NAME = "mech-baseline"  # shown on the spectator stream, cut to 32 chars by the server
_CONTINUOUS = ("move", "aim", "boost")  # what the server keeps when an answer is missing


# ----------------------------------------------------------------------------- decision code
class Bot:
    """Decision code. Same logic as example_bot.Bot (the baseline)."""

    name = BOT_NAME

    def __init__(self, hello: mf.Hello):
        self.reach = hello["config"]["loot"]["pickup_radius"]

    def act(self, obs: mf.Observation) -> mf.Action:
        me = obs["self"]
        x, y, z = me["pos"]
        goal = obs["zone"]["next_center"]
        act: mf.Action = {"tick": obs["tick"]}
        if obs["offer"]:
            act["pick"] = 0
        near = [p for p in obs["pickups"] if math.dist(p["pos"][:2], (x, y)) < self.reach]
        if me["weapons"][1] is None and near and near[0]["kind"] != "gun":
            act["take"] = near[0]["id"]
        enemies = sorted((m for m in obs["mechs"] if m["kind"] == "mech"), key=lambda m: math.dist(m["pos"], me["pos"]))
        yaw = math.atan2(goal[1] - y, goal[0] - x)
        act.update(move=[math.cos(yaw), math.sin(yaw)], aim=[yaw, 0.0], boost=not me["in_zone"], jump=obs["tick"] % 40 == 0)
        if enemies:
            t = enemies[0]["pos"]
            aim = [math.atan2(t[1] - y, t[0] - x), math.atan2(t[2] - z - 2.5, math.dist(t[:2], (x, y)))]
            act.update(aim=aim, fire=[True, True])
            mf.draw.line([x, y, z + 7], [t[0], t[1], t[2] + 4.5], "#ff5040")
        mf.draw.point([goal[0], goal[1], z], "#40c0ff", 6)
        mf.draw.label(f"hp {me['hp']:.0f} · {len(enemies)} in sight")
        return act


# ----------------------------------------------------------------------------- transport + safety net
class _Errors:
    """Rate-limited stderr logging so a bug that fires every tick cannot flood the log."""

    def __init__(self) -> None:
        self.count = 0

    def log(self, where: str) -> None:
        self.count += 1
        if self.count <= 5 or self.count % 100 == 0:
            mf.log(f"[bot] {where} failed ({self.count} errors so far):\n{traceback.format_exc()}")


def _write(msg: Dict[str, Any]) -> None:
    """One protocol line on stdout. allow_nan=False: NaN/inf would be invalid JSON (a bad line)."""
    data = (json.dumps(msg, separators=(",", ":"), allow_nan=False) + "\n").encode("utf-8")
    out = sys.stdout.buffer
    out.write(data)
    out.flush()


def _reader(q: "queue.Queue[Optional[bytes]]") -> None:
    try:
        for line in sys.stdin.buffer:
            if line.strip():
                q.put(line)
    except Exception:  # stdin closed under us: treat as EOF
        pass
    q.put(None)


def _safe_tick(obs: Dict[str, Any]) -> Any:
    return obs.get("tick", 0)


def main() -> int:
    errors = _Errors()
    q: "queue.Queue[Optional[bytes]]" = queue.Queue()
    threading.Thread(target=_reader, args=(q,), daemon=True).start()
    bot: Optional[Bot] = None
    last: Dict[str, Any] = {}  # continuous parts of the last good action

    while True:
        try:
            batch = [q.get()]
            while True:  # drain: only the newest observation matters (protocol.md section 2)
                try:
                    batch.append(q.get_nowait())
                except queue.Empty:
                    break
            latest: Optional[Dict[str, Any]] = None
            for raw in batch:
                if raw is None:
                    return 0  # stdin closed
                try:
                    msg = json.loads(raw)
                except ValueError:
                    continue
                if not isinstance(msg, dict):
                    continue
                kind = msg.get("type")
                if kind == "hello":
                    cfg = msg.get("config") or {}
                    mf.draw.caps = cfg.get("debug", {}) if isinstance(cfg, dict) else {}
                    try:
                        bot = Bot(msg)
                    except Exception:
                        errors.log("Bot.__init__")
                    _write({"type": "ready", "name": BOT_NAME[:32]})
                elif kind == "obs":
                    latest = msg
                elif kind == "end":
                    return 0
            if latest is None:
                continue

            tick = _safe_tick(latest)
            mf.draw.clear()
            action: Dict[str, Any]
            try:
                if bot is None:
                    raise RuntimeError("no bot (hello missing or Bot.__init__ failed)")
                action = dict(bot.act(latest) or {})
                action["tick"] = tick
                d = mf.draw.take()
                if d and "debug" not in action:
                    action["debug"] = d
                json.dumps(action, allow_nan=False)  # validate before committing to it
                last = {k: action[k] for k in _CONTINUOUS if k in action}
            except Exception:
                errors.log(f"act() at tick {tick}")
                mf.draw.clear()
                action = {**last, "tick": tick}  # safe default: coast on the last good move/aim/boost
            try:
                _write(action)
            except (TypeError, ValueError):
                errors.log(f"serialising the action at tick {tick}")
                _write({"tick": tick})
        except (BrokenPipeError, KeyboardInterrupt):
            return 0  # the server went away: nothing left to do
        except Exception:
            errors.log("main loop")  # never let the process die; keep answering


if __name__ == "__main__":
    try:
        code = main()
    except Exception:
        mf.log(f"[bot] fatal:\n{traceback.format_exc()}")
        code = 1
    try:
        sys.stdout.flush()
    except Exception:
        pass
    sys.exit(code)
