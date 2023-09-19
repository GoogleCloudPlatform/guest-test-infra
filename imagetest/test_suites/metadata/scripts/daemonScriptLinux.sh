#!/bin/bash

nohup sleep 3600 > /dev/null 2>&1 < /dev/null &
echo $! > /daemon_out.txt
