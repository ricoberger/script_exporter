#!/usr/bin/env bash

set -e

ping -c 3 $1 &> /dev/null
