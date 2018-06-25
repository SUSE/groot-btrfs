#!/bin/bash

set -e

source ./scripts/trap_handling.sh

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CERTS_DIR="test_registry_certs"

. $DIR/../make/include/colors.sh
printf "${OK_COLOR}==> groot-btrfs integration test suite ${NO_COLOR}\n"

compile() {
  make build
  chmod +x build/linux-amd64/*
  sudo chmod +s build/linux-amd64/
  sudo chown root:root build/linux-amd64/drax
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

  if [ "${3}" == "true" ]; then
    prepare_docker_conf
    echo "Using conf file test_docker_conf.yaml..."
    docker_conf=" -v ${DIR}/test_docker_conf.yaml:/test_docker_conf.yaml "
    conf_arg=" --config /test_docker_conf.yaml "
  fi

  #docker run --rm -a stdout \
  docker run --rm -a stdout -a stderr \
    -v $PWD:/workdir \
    -v $BTRFS:/btrfs \
    -v $DIR/../build/linux-amd64/drax:/bin/drax \
    -v $DIR/../build/linux-amd64/groot-btrfs:/bin/groot-btrfs \
    ${docker_conf} \
    -w '/workdir' \
    splatform/groot-btrfs-integration-tests \
    groot-btrfs \
      --drax-bin '/bin/drax' \
      --btrfs-progs-path '/sbin/' \
      --store-path /btrfs \
      ${conf_arg} \
      create --disk-limit-size-bytes 0 ${1} ${2}
}

delete_image() {
  echo "Deleting image ${1} using container ..."
  docker run --rm -a stdout \
    -v $PWD:/workdir \
    -v $BTRFS:/btrfs \
    -v $DIR/../build/linux-amd64/drax:/bin/drax \
    -v $DIR/../build/linux-amd64/groot-btrfs:/bin/groot-btrfs \
    -w '/workdir' \
    splatform/groot-btrfs-integration-tests \
    groot-btrfs \
      --drax-bin '/bin/drax' \
      --btrfs-progs-path '/sbin/' \
      --store-path /btrfs delete \
        $1 > /dev/null
}

image_stats() {
  sudo $DIR/../build/linux-amd64/groot-btrfs --drax-bin $DIR/../build/linux-amd64/drax --store-path $BTRFS stats ${1}
}

run_registry() {
  mkdir -p $CERTS_DIR
  pushd $CERTS_DIR > /dev/null
    # Create self signed certificates
    #openssl genrsa 1024 > domain.key
    #chmod 400 domain.key
    openssl req \
        -new \
        -newkey rsa:4096 \
        -days 365 \
        -nodes \
        -x509 \
        -subj "/C=US/ST=Test/L=Test/O=Test/CN=127.0.0.1" \
        -keyout domain.key \
        -out domain.crt
  popd > /dev/null

  docker run -d \
    --name test-registry \
    -v `pwd`/$CERTS_DIR:/certs \
    -e REGISTRY_HTTP_ADDR=0.0.0.0:5000\
    -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
    -e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
    -p 5000:5000 \
    registry:2

  printf "Waiting local docker registry to launch on 5000."
  total_secs=0
  while ! nc -z localhost 5000; do
    if $total_secs > 5; then
      fail_with_error "\nFailed to start docker registry after 5 seconds. Exiting"
    fi
    sleep 0.5
    printf "."
    total_secs+=0.5
  done
  printf "\nDocker registry launched\n"

  REGISTRY_IP=$(docker inspect test-registry | jq -r .[0].NetworkSettings.IPAddress)
}

function cleanup_registry {
  echo "Cleaning up registry and certs..."
  docker stop test-registry > /dev/null || docker kill test-registry > /dev/null
  docker rm test-registry
  rm -rf $CERTS_DIR
}
trap_add cleanup_registry EXIT

function prepare_docker_conf {
  cat << EOF > $DIR/test_docker_conf.yaml
---
#log_level: debug
insecure_registries:
  - ${REGISTRY_IP}:5000
EOF
trap_add "rm ${DIR}/test_docker_conf.yaml" EXIT
}

pushd $DIR/..
source ./scripts/prepare_test_btrfs
compile

create_image docker://library/busybox busybox

message="Testing the right amount of volumes were created."
expect '[ $(ls -lad $BTRFS/volumes | wc -l) == 1 ]'

message="Testing that image dependencies were written."
expect '[ $(cat $BTRFS/meta/dependencies/* | jq -r .[]) == $(ls $BTRFS/volumes) ]'

declare -a files=("bin/df" "bin/ifup" "bin/mv" "bin/uname" "bin/yes" "bin/wget"
                "etc/passwd"
                )
for file in "${files[@]}"; do
  message="Testing that the bundle contains ${file}."
  expect "[ -f '${BTRFS}/images/busybox/rootfs/$file' ]"
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
expect "[ -f ${BTRFS}/meta/bundle-busybox-metadata.json ]"

message="Testing that volume meta files exist."
expect "[ $(ls ${BTRFS}/meta/volume-${BUSYBOX_VOLUME} | wc -l) == 1 ]"

stats=$(image_stats busybox) || (echo $stats && exit $?)
message="Testing that 'stats' command works."
expect <<condition
[ -n $(echo $stats | jq -r '[."disk_usage"][]["total_bytes_used"]') ]
condition
expect <<condition
[ -n $(image_stats busybox | jq -r '[."disk_usage"][]["exclusive_bytes_used"]') ]
condition

delete_image busybox

message="Testing that image is deleted."
expect "[ ! -d ${BTRFS}/images/busybox ]"

message="Testing that volumes are no longer in dependencies."
expect "[ -z $(ls ${BTRFS}/meta/dependencies/) ]"

message="Testing that bundle meta file is deleted."
expect "[ ! -f ${BTRFS}/meta/bundle-busybox-metadata.json ]"

create_image docker://library/alpine alpine

message="Testing that volumes are removed upon next creation."
expect "[ -z $(ls ${BTRFS}/volumes | grep ${BUSYBOX_VOLUME}) ]"

message="Testing that volume meta files are deleted upon next creation."
expect "[ ! -f ${BTRFS}/meta/volume-${BUSYBOX_VOLUME} ]"

run_registry
docker pull busybox
docker tag busybox 127.0.0.1:5000/busybox
docker push 127.0.0.1:5000/busybox

message="Testing that pulling from insecure registries without an explicit whitelist is not possible"
expect "! create_image docker://${REGISTRY_IP}:5000/busybox busybox"

message="Testing that pulling from insecure registries with an explicit whitelist is not possible"
expect "create_image docker://${REGISTRY_IP}:5000/busybox busybox true"

popd > /dev/null
