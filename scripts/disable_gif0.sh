#!/bin/sh
set -eu

echo "== Before =="
ifconfig gif0 || true

echo
echo "== Disable gif0 =="
sudo ifconfig gif0 down

echo
echo "== After =="
ifconfig gif0 || true
