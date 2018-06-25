#!/bin/bash

set -e

source ./scripts/trap_handling.sh

export DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export CERTS_DIR="test_registry_certs"

. $DIR/../make/include/colors.sh
printf "${OK_COLOR}==> groot-btrfs parallel image creation script ${NO_COLOR}\n"

compile() {
  make build
  chmod +x build/linux-amd64/*
  sudo chmod +s build/linux-amd64/
  sudo chown root:root build/linux-amd64/drax
}

function prepare_docker_conf {
  cat << EOF > $DIR/test_docker_conf.yaml
---
#log_level: debug
insecure_registries:
  - ${REGISTRY_IP}:5000
EOF
trap_add "rm ${DIR}/test_docker_conf.yaml" EXIT
}

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
    -v $DIR/../locks/:/tmp/groot-locks \
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
      create --disk-limit-size-bytes 0 $1 $2
}
export -f create_image

pushd $DIR/..
source ./scripts/prepare_test_btrfs
compile

sudo rm -rf ${DIR}/results

set -x
parallel --results ${DIR}/results/ -N2 create_image docker://{1} {2} ::: "library/busybox" "busybox" "library/alpine" "alpine" "viovanov/node-env-tiny" "node-env-tiny"

set +x

echo "All image created"
