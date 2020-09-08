#!/bin/bash

brctl addbr br0
brctl addbr vbr1
brctl addbr vbr2

brctl addif br0 ens33
ifconfig ens33 0
ifconfig br0 192.168.1.109
ifconfig vbr1 10.10.1.254/24
ifconfig vbr2 10.10.2.254/24
route add default gw 192.168.1.1
