#!/bin/bash

set -e

if [ "$#" != 1 ]; then
  echo "USAGE $0 option<deploy | run>"
  exit 1
fi
option="$1"
zip_file_name="application.zip"
app_file_name="chat-server"

deploy() {
  cd /tmp || exit
  unzip "${zip_file_name}" -d ~/
  cd ~ || exit
  chmod +x "${app_file_name}"
}

run() {
  cd ~/
  ./"${app_file_name}" --data-dir=./data &
  sleep 5
}

if [ "$option" = 'deploy' ]; then
  deploy
elif [ "$option" = 'run' ]; then
  run
else
  deploy
  run
fi
