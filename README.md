# Log Collector

## Building
```
CGO_ENABLED=0 go build -a -installsuffix cgo -o log-collector .

docker build -t coco/log-collector .
```

## Description
The `log-collector` is a golang application that posts a stdin to S3 in order to be processed by the `resilient-splunk-forwarder`.
Docker image builds a container that stores the journalctl logs to S3.
 
## Usage ex
e.g. journalctl -f --output=json | ./log-collector -env=$ENV -workers=$WORKERS -buffer=$BUFFER -batchsize=$BATCHSIZE -batchtimer=$BATCHTIMER -bucketName=$BUCKET_NAME -awsRegion=$AWS_REGION
