#!/bin/bash

sudo apt update
sudo apt install curl make golang -y

curl -fsSL https://opencode.ai/install | bash

curl -sL https://raw.githubusercontent.com/wimpysworld/deb-get/main/deb-get | sudo -E bash -s install deb-get
deb-get install zenith
deb-get update
deb-get upgrade -y
sudo apt upgrade -y

source $HOME/.bashrc
