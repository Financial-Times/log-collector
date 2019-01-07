FROM golang:1.11-alpine

ENV PROJECT=log-collector

# Include code
ENV ORG_PATH="github.com/Financial-Times"
ENV SRC_FOLDER="${GOPATH}/src/${ORG_PATH}/${PROJECT}"
COPY . ${SRC_FOLDER}
WORKDIR ${SRC_FOLDER}

# Set up our extra bits in the image
RUN apk --no-cache add git curl
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Install dependancies and build app
RUN $GOPATH/bin/dep ensure -vendor-only
RUN BUILDINFO_PACKAGE="${ORG_PATH}/${PROJECT}/vendor/${ORG_PATH}/service-status-go/buildinfo." \
    && VERSION="version=$(git describe --tag --always 2> /dev/null)" \
    && DATETIME="dateTime=$(date -u +%Y%m%d%H%M%S)" \
    && REPOSITORY="repository=$(git config --get remote.origin.url)" \
    && REVISION="revision=$(git rev-parse HEAD)" \
    && BUILDER="builder=$(go version)" \
    && LDFLAGS="-s -w -X '"${BUILDINFO_PACKAGE}$VERSION"' -X '"${BUILDINFO_PACKAGE}$DATETIME"' -X '"${BUILDINFO_PACKAGE}$REPOSITORY"' -X '"${BUILDINFO_PACKAGE}$REVISION"' -X '"${BUILDINFO_PACKAGE}$BUILDER"'" \
    && CGO_ENABLED=0 go build -a -o /artifacts/${PROJECT} -ldflags="${LDFLAGS}"


# Multi-stage build - copy only the certs and the binary into the image
FROM alpine:3.8
WORKDIR /
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /artifacts/* /

# Adding the utilities need for running journalctl & kubectl from inside the container.
# This version has to be compatible with the one running on the clusters, that is configured here: https://github.com/Financial-Times/content-k8s-provisioner/blob/master/ansible/vars/defaults.yaml#L3
ENV KUBECTL_VERSION="v1.11.0"

ADD https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl /usr/local/bin/kubectl

RUN apk add --no-cache bash gawk sed grep bc coreutils jq ca-certificates && \
  chmod +x /usr/local/bin/kubectl && \
  # Basic check that kubectl works.
  kubectl version --client

CMD exec /log-collector -env=$ENV -workers=$WORKERS -buffer=$BUFFER -batchsize=$BATCHSIZE -batchtimer=$BATCHTIMER -bucketName=$BUCKET_NAME -awsRegion=$AWS_REGION -dnsAddress=$DNS_ADDRESS
