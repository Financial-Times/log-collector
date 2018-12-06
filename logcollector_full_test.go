package main

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/log-collector/filter"
	"github.com/Financial-Times/log-collector/forwarder"
)

type s3ServiceMock struct {
	sync.RWMutex
	cache []string
}

var s3Mock = &s3ServiceMock{}

func (s3 *s3ServiceMock) Put(obj string) error {
	obj = strings.Replace(obj, "dispatch", "safe", -1)
	obj = strings.Replace(obj, "error", "dispatch", -1)
	s3.Lock()
	s3.cache = append(s3.cache, obj)
	s3.Unlock()
	return nil
}

func init() {
	forwarder.Env = "dummy"
	forwarder.Workers = 8
	forwarder.ChanBuffer = 256
	forwarder.Batchsize = 10
	forwarder.Batchtimer = 5
	forwarder.Bucket = "testbucket"

	forwarder.NewS3Service = func(string, string, string) (forwarder.S3Service, error) {
		return s3Mock, nil
	}

	filter.Env = forwarder.Env
	filter.DNSAddress = "dummy"
}

var logEntry = `
{
  "PRIORITY": "6",
  "_TRANSPORT": "journal",
  "_PID": "1559",
  "_UID": "0",
  "_GID": "0",
  "_COMM": "dockerd",
  "_EXE": "/run/torcx/unpack/docker/bin/dockerd",
  "_CMDLINE": "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}} --mtu=8951",
  "_CAP_EFFECTIVE": "3fffffffff",
  "_SELINUX_CONTEXT": "system_u:system_r:kernel_t:s0",
  "_SYSTEMD_CGROUP": "/system.slice/docker.service",
  "_SYSTEMD_UNIT": "docker.service",
  "_SYSTEMD_SLICE": "system.slice",
  "_SYSTEMD_INVOCATION_ID": "2ca152c8fa55437abd5b47bca66abb44",
  "_MACHINE_ID": "ec29031771b92b37b4c5f2594b4d143d",
  "_HOSTNAME": "ip-10-172-32-220.eu-west-1.compute.internal",
  "CONTAINER_ID_FULL": "582410529ff532038409f0e7a07fd0dbb8dea2c788b9f714ad63ded6561585ef",
  "CONTAINER_NAME": "k8s_methode-article-internal-components-mapper_methode-article-internal-components-mapper-55599f64dd-7d4cr_default_e1ce147e-f393-11e8-86a5-06444a6b86d6_0",
  "CONTAINER_TAG": "coco/methode-article-internal-components-mapper@sha256:6d2ad9e499dc72060ff1b344505626539c2a940da9219aa15b4671f5a6d27a3b",
  "SYSLOG_IDENTIFIER": "coco/methode-article-internal-components-mapper@sha256:6d2ad9e499dc72060ff1b344505626539c2a940da9219aa15b4671f5a6d27a3b",
  "CONTAINER_ID": "582410529ff5",
  "MESSAGE": "10.2.31.47 -  -  [29/Nov/2018:14:40:39 +0000] \"Message",
  "_SOURCE_REALTIME_TIMESTAMP": "1543502439189782"
}
`

func Test_FullCollector(t *testing.T) {
	in, out := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(1)

	logsReader = in
	go func() {
		main()
		wg.Done()
	}()

	messageCount := 100
	for i := 0; i < messageCount; i++ {
		out.Write([]byte(logEntry + "\n"))
	}

	if err := out.Close(); err != nil {
		assert.Fail(t, "Error closing the pipe writer %v", err)
	}
	if waitTimeout(&wg, 2*time.Second) {
		assert.Fail(t, "Whole flow should have been stopped on pipe close")
	}

	s3Mock.RLock()
	l := len(s3Mock.cache)
	s3Mock.RUnlock()

	assert.Equal(t, messageCount/forwarder.Batchsize, l)
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
