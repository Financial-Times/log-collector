package forwarder

import (
	"flag"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestMain(m *testing.M) {
	Env = "dummy"
	Workers = 8
	ChanBuffer = 256
	Batchsize = 10
	Batchtimer = 5
	Bucket = "testbucket"

	flag.Parse()

	NewS3Service = func(string, string, string) (S3Service, error) {
		return s3Mock, nil
	}

	os.Exit(m.Run())
}

func Test_Forwarder(t *testing.T) {
	in, out := io.Pipe()

	go Forward(in)
	messageCount := 100
	for i := 0; i < messageCount; i++ {
		out.Write([]byte(`127.0.0.1 - - [21/Apr/2015:12:15:34 +0000] "GET /eom-file/all/e09b49d6-e1fa-11e4-bb7f-00144feab7de HTTP/1.1" 200 53706 919 919` + "\n"))
	}
	e := out.Close()
	if e != nil {
		assert.Fail(t, "Error closing the pipe writer %v", e)
	}
	time.Sleep(5 * time.Second)

	s3Mock.RLock()
	l := len(s3Mock.cache)
	s3Mock.RUnlock()

	assert.Equal(t, messageCount/Batchsize, l)
}
