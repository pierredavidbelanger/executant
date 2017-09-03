# executant

A simple Go program that watch a key in Consul for change,
on update, this program write the filtered docker-compose yml value
to a folder and docker-compose it up (or down if the key is absent or its value empty).

## usage

```sh
docker run \
    --net host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/lib/executant:/var/lib/executant \
    pierredavidbelanger/executant
```

This will connect to Consul on `localhost:8500` (this is why we need `--net host`),
watch the `executant.yml` key for change.
On change, it will create a `/var/lib/executant/docker-compose.yml` file (in the container AND in the host because of the `-v`),
filter out services without the `executant.enabled=true`,
then run `docker-compose up` (or `down` if the value is empty),
and since we mounted `/var/run/docker.sock` in the container, the services containers will be created on the host's docker.

## config

Here are the `ENV` var available, and their default value:

```yml
# Consul local agent address
CONSUL_HTTP_ADDR: http://localhost:8500

# The key to watch
EXECUTANT_KEY: executant.yml

# The service filters
EXECUTANT_FILTERS: executant.enabled=true

# the container local (and symetricly host mounted) folder where to put the project files
EXECUTANT_WORK_DIR: /var/lib/executant
```

see [Consul API](https://github.com/hashicorp/consul/blob/master/api/api.go) for other ENV var
