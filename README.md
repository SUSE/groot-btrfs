This is an image plugin for btrfs that implements the Driver interface of [groot](https://github.com/cloudfoundry/groot).

## Build

Just call `make build`

## Testing

In order to run the tests you will need:

- a working Go development environment (if `make build` succeeds then you are good to go)
- working docker (If you can do `docker images` then you are good to go)
- btrfs kernel module loaded

If you have all the above, just do a `make test`.

## Running manually

You can experiment with this driver but running it manually (outside CloudFoundry). You can find some examples in
[the integration tests](integration_tests/suite.sh).

The process is as follows:

- You initialize a store using the [init-store script](scripts/init-store). The simplest version of that command would be something like this:

```
sudo ./scripts/init-store -s /tmp/mybtrfs -b 1G
```

run the script with no arguments for more information.

- You then create a bundle with a command like this:

```
./groot-btrfs --drax-bin=/bin/drax --btrfs-progs-path /sbin/ --store-path /tmp/mybtrfs create docker://library/busybox busybox
```

- You delete the store:

```
sudo scripts/delete-store /tmp/mybtrfs
```
