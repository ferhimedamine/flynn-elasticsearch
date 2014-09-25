#!/bin/bash

case $1 in
  elastic)
    shift
    exec /bin/flynn-elasticsearch $*
    ;;
  *)
    echo "Usage: $0 {elastic}"
    exit 2
    ;;
esac
