#!/bin/bash
#
set -e

DEPS="http bencode taipei"
for dep in ${DEPS}; do
	cd $dep ; make clean ; cd ..
done
