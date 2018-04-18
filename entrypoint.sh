#!/bin/bash

[ -z "$GITHUB_TOKEN" ] || \
  git config --global \
  url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf \
  "https://github.com/"

[ -z "$GITHUB_WIKI_URL" ] && { echo "Need to set GITHUB_WIKI_URL"; exit 1; }
GITHUB_MIRROR_FREQUENCY=${GITHUB_MIRROR_FREQUENCY:-600}
DOCROOT="/tmp/wiki"

update_wiki(){
  while true
  do
    cd $DOCROOT
    git pull
    sleep $GITHUB_MIRROR_FREQUENCY
  done
}

[ -z "$DOCROOT" ] || FLAGS="-docroot $DOCROOT "
[ -z "$PREFIX" ] || FLAGS+="-prefix $PREFIX "

if [ -d $DOCROOT ]; then
  rm -rf $DOCROOT
fi

if [ ! -d $DOCROOT ]; then
  mkdir -p $DOCROOT
fi

git clone $GITHUB_WIKI_URL $DOCROOT

# Background updater
update_wiki &


# Replace with rowi daemon
rowi $FLAGS
