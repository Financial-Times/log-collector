# Log Collector

[![Circle CI](https://circleci.com/gh/Financial-Times/log-collector/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/log-collector/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/log-collector)](https://goreportcard.com/report/github.com/Financial-Times/log-collector) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/log-collector/badge.svg)](https://coveralls.io/github/Financial-Times/log-collector)

## Introduction
The `log-collector` is a golang application that fetches JSON log messages from stdin, filters and enrich them and then forwards them to S3 in order to be processed by the `resilient-splunk-forwarder`.
Docker image builds a container that stores the journalctl logs to S3.

## Building
```
        curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
        go get -u github.com/Financial-Times/log-collector
        cd $GOPATH/src/github.com/Financial-Times/log-collector
        dep ensure
        go build .
```

## Running locally

1. Run the tests and install the binary:

        dep ensure
        go test -race ./...
        go install

2. Run the binary
    ```
    e.g. journalctl -f --output=json | ./log-collector -env=$ENV -workers=$WORKERS -buffer=$BUFFER -batchsize=$BATCHSIZE -batchtimer=$BATCHTIMER -bucketName=$BUCKET_NAME -awsRegion=$AWS_REGION -dnsAddress=$AWS_DNS_ADDRESS
    ```

## Running in Kubernetes
On a Kubernetes cluster, the service runs as a `Daemonset`, so that a pod is kept running on every node to collect the logs.

### How it works

Here are the key parts:

1. For reading the logs from journald, the `journalctl` executable is mounted in the pod from the host, together with everything else needed for accessing the logs.
   We're doing this, and not copy a specific journalctl version through the Dockerfile, in order to make sure that we're using the journalctl version that surely works from the host.
   Checkout the [deamonset start command](helm/log-collector/templates/daemonset.yaml#L84) and [mounts](helm/log-collector/templates/daemonset.yaml#L101) for details
1. For not losing logs from a node, whenever the pod terminates for whatever reason, the shutdown time for that node is recorded in the Config Map `log-collector-stop-time`.
    The next time the `log-collector` starts on the node, it will resume the logs from the value written in the configmap.
    The ConfigMap looks like:

       ```
       kind: ConfigMap
       metadata:
         name: log-collector-stop-time
       apiVersion: v1
       data:
         ip-10-172-32-11.eu-west-1.compute.internal: 2018-12-03 09:14:47.268
         ip-10-172-32-21.eu-west-1.compute.internal: 2018-12-03 09:14:07.527
         ...
       ```

    For details, checkout the [pre-stop lifecycle hook comand](helm/log-collector/templates/daemonset.yaml#L87) and the [container start script](helm/log-collector/templates/daemonset.yaml#L76)
1. For being able to use `kubectl` for accomplishing the previous step, we're adding the executable to the docker image through the [Dockerfile](Dockerfile#L36).
    We don't need any other additional setup (kubeconfig file) for being able to run `kubectl` commands from inside the pod.

