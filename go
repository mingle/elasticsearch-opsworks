#!/bin/bash -eu

DIR=$(cd `dirname ${BASH_SOURCE}` > /dev/null && pwd)

$DIR/init_rbenv

if ! (type rbenv > /dev/null 2>&1); then
  # initialize rbenv for this shell session
  export PATH="$HOME/.rbenv/bin:$PATH"
  eval "$(rbenv init -)"
fi

rake $*
