#!/bin/sh
set -eu

echo "[react-frontend] scaffold container is up on port 5173."
echo "[react-frontend] implement Vite React dashboard in next phase."

# Keep container alive until real frontend app is added.
tail -f /dev/null
