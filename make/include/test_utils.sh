mount_storage() {
  # Configure cgroup
  mount -t tmpfs cgroup_root /sys/fs/cgroup
  mkdir -p /sys/fs/cgroup/devices
  mkdir -p /sys/fs/cgroup/memory

  mount -tcgroup -odevices cgroup:devices /sys/fs/cgroup/devices
  devices_mount_info=$(cat /proc/self/cgroup | grep devices)
  devices_subdir=$(echo $devices_mount_info | cut -d: -f3)
  echo 'b 7:* rwm' > /sys/fs/cgroup/devices/devices.allow
  echo 'b 7:* rwm' > /sys/fs/cgroup/devices${devices_subdir}/devices.allow

  mount -tcgroup -omemory cgroup:memory /sys/fs/cgroup/memory

  # Setup loop devices
  for i in {0..256}
  do
    mknod -m777 /dev/loop$i b 7 $i
  done

  for i in {1..5}
  do
    # Make BTRFS Volume
    truncate -s 1G /btrfs_volume_${i}
    mkfs.btrfs --nodesize 4k -s 4k /btrfs_volume_${i}

    # Mount BTRFS
    mkdir /mnt/btrfs-${i}
    mount -t btrfs -o user_subvol_rm_allowed,rw /btrfs_volume_${i} /mnt/btrfs-${i}
    chmod 777 -R /mnt/btrfs-${i}
    btrfs quota enable /mnt/btrfs-${i}
  done
}

unmount_storage() {
  for i in {1..5}
  do
    umount -l /mnt/btrfs-${i}
  done
}

sudo_mount_storage() {
  local MOUNT_STORAGE_FUNC=$(declare -f mount_storage)
  sudo bash -c "$MOUNT_STORAGE_FUNC; mount_storage"
}

sudo_unmount_storage() {
  local UNMOUNT_STORAGE_FUNC=$(declare -f unmount_storage)
  sudo bash -c "$UNMOUNT_STORAGE_FUNC; unmount_storage"
}

install_dependencies() {
  if ! [ -d vendor ]; then
    glide install
  fi
}

setup_drax() {
  drax_path=$1
  cp $drax_path /usr/local/bin/drax
  chown root:root /usr/local/bin/drax
  chmod u+s /usr/local/bin/drax
}

sudo_setup_drax() {
  drax_path=$1

  local SETUP_DRAX_FUNC=$(declare -f setup_drax)
  sudo bash -c "$SETUP_DRAX_FUNC; setup_drax $drax_path"
}