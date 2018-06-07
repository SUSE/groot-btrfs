#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

. $DIR/../make/include/colors.sh
printf "${OK_COLOR}==> groot-btrfs integration test suite ${NO_COLOR}\n"

compile() {
  go build -o groot-btrfs
  chmod +x groot-btrfs
  go build -o drax/drax github.com/SUSE/groot-btrfs/drax
}


test_with_message() {
  printf "${OK_COLOR} ${1} ${NO_COLOR}\n"
}

succeed_with_message() {
  printf "${OK_COLOR} ${1} ${NO_COLOR}\n"
}

fail_with_error() {
  printf "${ERROR_COLOR} ${1} ${NO_COLOR}\n"
  exit 1
}

expect() {
  if eval $1; then
    succeed_with_message "${message}"
  else
    fail_with_error "${message}. Failed on condition: '${1}'"
  fi
}

# Calls groot-btrfs create command for the image specified by $1
# The command is run inside a container to have predictable locations for
# btrfs-progs and drax.
create_image() {
  echo "Creating image ${1} using container ..."
  docker run -a stdout \
    -v $PWD:/workdir \
    -v $BTRFS:/btrfs \
    -v $DIR/../drax/drax:/bin/drax \
    -w '/workdir' \
    splatform/groot-btrfs-integration-tests \
    ./groot-btrfs \
      --drax-bin '/bin/drax' \
      --btrfs-progs-path '/sbin/' \
      --store-path /btrfs create \
        --disk-limit-size-bytes 0 \
        $1 test_image > /dev/null
}

delete_image() {
  echo "Deleting image ${1} using container ..."
  docker run -a stdout \
    -v $PWD:/workdir \
    -v $BTRFS:/btrfs \
    -v $DIR/../drax/drax:/bin/drax \
    -w '/workdir' \
    splatform/groot-btrfs-integration-tests \
    ./groot-btrfs \
      --drax-bin '/bin/drax' \
      --btrfs-progs-path '/sbin/' \
      --store-path /btrfs delete \
        $1 > /dev/null
}

image_stats() {
  sudo chmod +s $DIR/../drax/drax > /dev/null
  sudo chown root:root $DIR/../drax/drax > /dev/null
  sudo $DIR/../groot-btrfs --drax-bin $DIR/../drax/drax --store-path $BTRFS stats test_image
}

pushd $DIR/..
source ./scripts/prepare_test_btrfs
compile

create_image docker://library/busybox

message="Testing the right amount of volumes were created."
expect '[ $(ls -lad $BTRFS/volumes | wc -l) == 1 ]'

message="Testing that image dependencies were written."
expect '[ $(cat $BTRFS/meta/dependencies/* | jq -r .[]) == $(ls $BTRFS/volumes) ]'

declare -a files=("bin/df" "bin/ifup" "bin/mv" "bin/uname" "bin/yes" "bin/wget"
                "etc/passwd"
                )
for file in "${files[@]}"; do
  message="Testing that the bundle contains ${file}."
  expect "[ -f '${BTRFS}/images/test_image/rootfs/$file' ]"
done

message="Testing that uid mappings were written in namespace.json"
expect <<condition
[ $(cat ${BTRFS}/meta/namespace.json | jq -r '["uid-mappings"][]') == '0:1000:1 ]"
condition

message="Testing that gid mappings were written in namespace.json"
expect <<condition
[ $(cat ${BTRFS}/meta/namespace.json | jq -r '["gid-mappings"][]') == '0:100:1 ]"
condition

BUSYBOX_VOLUME=$(ls ${BTRFS}/volumes)

message="Testing that bundle meta file exists."
expect "[ -f ${BTRFS}/meta/bundle-test_image-metadata.json ]"

message="Testing that volume meta files exist."
expect "[ $(ls ${BTRFS}/meta/volume-${BUSYBOX_VOLUME} | wc -l) == 1 ]"

stats=$(image_stats test_image) || (echo $stats && exit $?)
message="Testing that 'stats' command works."
expect <<condition
[ -n $(echo $stats | jq -r '[."disk_usage"][]["total_bytes_used"]') ]
condition
expect <<condition
[ -n $(image_stats test_image | jq -r '[."disk_usage"][]["exclusive_bytes_used"]') ]
condition

delete_image test_image

message="Testing that image is deleted."
expect "[ ! -d ${BTRFS}/images/test_image ]"

message="Testing that volumes are no longer in dependencies."
expect "[ -z $(ls ${BTRFS}/meta/dependencies/) ]"

message="Testing that bundle meta file is deleted."
expect "[ ! -f ${BTRFS}/meta/bundle-test_image-metadata.json ]"

create_image docker://library/alpine

message="Testing that volumes are removed upon next creation."
expect "[ -z $(ls ${BTRFS}/volumes | grep ${BUSYBOX_VOLUME}) ]"

message="Testing that volume meta files are deleted upon next creation."
expect "[ ! -f ${BTRFS}/meta/volume-${BUSYBOX_VOLUME} ]"

popd > /dev/null
