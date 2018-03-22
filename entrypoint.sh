#!/bin/bash

[ -z "$GITHUB_TOKEN" ] || \
  git config --global \
  url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf \
  "https://github.com/"

[ -z "$GITHUB_WIKI_URL" ] && { echo "Need to set GITHUB_WIKI_URL"; exit 1; }
GITHUB_MIRROR_FREQUENCY=${GITHUB_MIRROR_FREQUENCY:-600}
WORKDIR="/tmp/wiki"

update_wiki(){
  while true
  do
    cd $WORKDIR
    git pull
    sleep $GITHUB_MIRROR_FREQUENCY
  done
}

if [ -d $WORKDIR ]; then
  rm -rf $WORKDIR
fi

if [ ! -d $WORKDIR ]; then
  mkdir -p $WORKDIR
fi

git clone $GITHUB_WIKI_URL $WORKDIR

# Background updater
update_wiki &


# Replace with rowi daemon
rowi -docroot $WORKDIR
