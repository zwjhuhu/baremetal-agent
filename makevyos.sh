#!/bin/bash
IMGDIR=/home/cz-cloud
make clean tar
bash mkvyos.sh $IMGDIR/vyos-agent.qcow2 target/zvr.tar.gz init.sh