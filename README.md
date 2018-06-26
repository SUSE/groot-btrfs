
# Testing

1. Run it on openSUSE

2. Be root

3. Make sure you have creds that allow you to download `docker://viovanov/test`

4. Make sure you have a `groot` user (`sudo useradd groot`)

5. Make sure you don't have a shared root mount.

If running `grep -iP '/ /\s' /proc/$$/mountinfo` has 'shared' in the output, run
`mount --make-private /` before running tests.


## Unit tests

```
DOCKER_REGISTRY_USERNAME=*** DOCKER_REGISTRY_PASSWORD=*** make test
```

## Integration Tests

```
DOCKER_REGISTRY_USERNAME=*** DOCKER_REGISTRY_PASSWORD=*** make integration
```

# Building

```
make build
```

You'll find the binaries in `./build/linux-amd64/`.
