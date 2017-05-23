# executant

A simple Go program that watch some keys in Consul for change,
on key update this program write the docker-compose yml value 
to a folder and docker-compose it up (or down if the key is absent or its value empty).

## usage

```sh
docker run \
    --net host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/lib/executant:/var/lib/executant \
    pierredavidbelanger/executant \
    myproject1 myproject2
```

This will connect to Consul on `localhost:8500` (this is why we need `--net host`), watch the `myproject1` and `myproject2` keys for change. On change, it will create an `[KEY]/docker-compose.yml` in `/var/lib/executant` (in the container AND in the host because of the `-v`), then run `docker-compose up` (or `down` if the value is empty), and since we mounted `/var/run/docker.sock` in the container, the services containers will be created on the host's docker.

## config

Here are the `ENV` var available, and their default value:

```yml
# Consul local agent address (see [Consul API](https://github.com/hashicorp/consul/blob/master/api/api.go) for other ENV var)
CONSUL_HTTP_ADDR: http://localhost:8500

# debug, info, warn, error, fatal, panic (see [Logrus API](https://github.com/sirupsen/logrus/blob/master/logrus.go))
LOG_LEVEL: info

# the container local (and symetricly host mounted) folder where to put the project files
EXECUTANT_WORK_DIR: /var/lib/executant
```