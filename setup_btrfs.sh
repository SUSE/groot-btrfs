#!/bin/bash


# Usage:
# sudo  ./setup_btrfs.sh /var/lib/grootfs/btrfs/store /var/lib/grootfs/btrfs/mounted
# sudo ./groot-btrfs --store-path /var/lib/grootfs/btrfs/mounted/  --btrfs-progs-path /sbin create docker://registry.hub.docker.com/library/busybox busybox
set -e

# Takes a path (to a file) as an argument and creates a btrfs filsystem on it
# The file will be created, the directory must exist.
setup_filesystem() {
  truncate -s 1G $1
  mkfs.btrfs $1
}

# Mounts the btrfs filesystem in $1 to the path in $2
# The $1 will probably be the one that was previously passed to setup_filesystem
# function.
mount_filesystem() {
  modprobe btrfs
  mkdir -p $2
  mount -t btrfs -o user_subvol_rm_allowed,noatime $1 $2
  chmod 777 $2
  btrfs quota enable $2
}

# Unmounts the btrfs filesystem mounted on $2 and deletes the directory. It
# also deletes the filesystem file on $1
cleanup_filesystem() {
  if mount | grep $2 > /dev/null; then
    echo "Unmounting $2"
    umount $2
  fi
  rm -rf $2
  rm -rf $1
}

cleanup_filesystem $1 $2
setup_filesystem $1
mount_filesystem $1 $2
mkdir $2/volumes
mkdir -p $2/meta/dependencies

echo "Setup complete. Example command:"
echo "./groot-btrfs --btrfs-progs-path /sbin create docker://registry.hub.docker.com/library/busybox busybox"
