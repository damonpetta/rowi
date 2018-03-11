#!/bin/bash

[ -z "$GITHUB_WIKI_URL" ] && { echo "Need to set GITHUB_WIKI_URL"; exit 1; }
GITHUB_MIRROR_FREQUENCY=${GITHUB_MIRROR_FREQUENCY:-600}

update_wiki(){
  while true
  do
    cd /app/wiki
    git pull
    sleep $GITHUB_MIRROR_FREQUENCY
  done
}

git clone $GITHUB_WIKI_URL /app/wiki

# Background updater
update_wiki &


# Replace with rowi daemon
sleep 1000000000
