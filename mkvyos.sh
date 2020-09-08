#!/bin/bash

export LIBGUESTFS_BACKEND=direct

which guestfish > /dev/null
if [ $? -ne 0 ]; then
   echo "guestfish is not installed"
   exit 1
fi

which qemu-img > /dev/null
if [ $? -ne 0 ]; then
   echo "qemu-img is not installed"
   exit 1
fi

usage() {
   echo "
USAGE:
$0 path_to_image path_to_zvr_tar path_to_initsh_path"
}

if [ -z $1 ]; then
   echo "missing parameter path_to_image"
   usage
   exit 1
fi

if [ ! -f $1 ]; then
   echo "cannot find the image"
   exit 1
fi

if [ -z $2 ]; then
   echo "missing parameter path_to_zvr_tar"
   usage
   exit 1
fi

if [ ! -f $2 ]; then
   echo "cannot find the zvr.tar.gz"
   exit 1
fi

if [ -z $3 ]; then
   echo "missing parameter path_to_initsh_path"
   usage
   exit 1
fi

if [ ! -f $3 ]; then
   echo "cannot find path_to_initsh"
   exit 1
fi

imgfile=$1
isVmdk=0
if echo $1 | grep -q -i '\.vmdk$'; then
    isVmdk=1
    imgfile=${1%%.vmdk}.qcow2
    qemu-img convert -f vmdk -O qcow2 "$1" "$imgfile"
fi

set -e

tmpdir=$(mktemp -d)

atexit() {
    /bin/rm -fr $tmpdir
    [ $isVmdk -eq 1 ] && /bin/rm -f $imgfile || true
}

trap atexit EXIT SIGHUP SIGINT SIGTERM

guestfish <<_EOF_
add $imgfile
run
mount /dev/sda1 /
upload $2 /boot/grub/zvr.tar.gz
upload $3 /boot/grub/init.sh
download /boot/grub/grub.cfg /tmp/grub.cfg
! sed -e 's/^set[[:space:]]\+timeout[[:space:]]*=[[:space:]]*[[:digit:]]\+/set timeout=0/g' -e '/^echo.*Grub menu/,/^fi$/d' /tmp/grub.cfg > /tmp/grub.cfg.new
upload /tmp/grub.cfg.new /boot/grub/grub.cfg
_EOF_

/bin/rm -rf $tmpdir

if [ $isVmdk -eq 1 ]; then
    /bin/rm -f "$1"
    qemu-img convert -f qcow2 -O vmdk "$imgfile" "$1"
fi

echo "successfully installed $2 to vyos image $1"
