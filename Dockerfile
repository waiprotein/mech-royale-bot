# Build from this folder: docker build -t my-bot .
# The match runs it with no network, a read-only root (writable /tmp), 1 CPU, 512 MB, uid 65534, and the map at /arena.
# Base image pinned to an exact patch release (Docker Hub tag pushed 2026-11-08).
# For a fully immutable pin, append the index digest after verifying it with
#   docker buildx imagetools inspect python:3.13.18-slim
# (Docker Hub reported sha256:e48f59cdba69a3cbd9ed2cf0f348d90d45a8d350d98134cf1070bc106ee9ed90 on 2026-11-21).
FROM python:3.13.18-slim
WORKDIR /bot
COPY . .
# Precompile as root at build time: the runtime FS is read-only, so Python cannot write __pycache__ later.
RUN python3 -m compileall -q /bot
ENTRYPOINT ["python3", "-u", "/bot/bot.py"]
