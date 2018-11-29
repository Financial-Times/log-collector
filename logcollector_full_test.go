package main

import (
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/log-collector/forwarder"
	"github.com/Financial-Times/log-collector/logfilter"
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

	logfilter.Env = forwarder.Env
	logfilter.DnsAddress = "dummy"
}

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
		out.Write([]byte(`127.0.0.1 - - [21/Apr/2015:12:15:34 +0000] "GET /eom-file/all/e09b49d6-e1fa-11e4-bb7f-00144feab7de HTTP/1.1" 200 53706 919 919` + "\n"))
	}

	if err := out.Close(); err != nil {
		assert.Fail(t, "Error closing the pipe writer %v", err)
	}
	wg.Wait()

	s3Mock.RLock()
	l := len(s3Mock.cache)
	s3Mock.RUnlock()

	assert.Equal(t, messageCount/forwarder.Batchsize, l)
}
