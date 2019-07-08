[![Docker Cloud Automated build](https://img.shields.io/docker/cloud/automated/macrat/hashcheck-exporter.svg)](https://hub.docker.com/r/macrat/hashcheck-exporter)
[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/macrat/hashcheck-exporter.svg)](https://hub.docker.com/r/macrat/hashcheck-exporter/builds)
[![GitHub](https://img.shields.io/github/license/macrat/hashcheck-exporter.svg)](https://github.com/macrat/hashcheck-exporter/blob/master/LICENSE)

hashcheck-exporter
==================

Web page change watcher for Prometheus.

## usage

### make config file
``` yaml
workers: 3  # number of workers for downloading.

targets:
  - url: https://example.com/path/to/target
  - url: https://example.com
    hash: 3587cb776ce0e4e8237f215800b7dffba0f25865cb84550e87ea8bbac838c423  # will check is SHA256 hash of content that downloaded from web server have same as this hash.
```

### execute exporter
#### from source
``` shell
$ go get github.com/macrat/hashcheck-exporter
$ hashcheck-exporter -config=/path/to/config.yml
```

#### with docker
``` shell
$ docker run -p 9998:9998 -v /path/to/config.yml:/app/hashcheck.yml hashcheck-exporter
```

### configure Prometheus
``` yaml
scrape_configs:
  - job_name: hashcheck
    static_configs:
      - targets:
        - localhost:9998
```
