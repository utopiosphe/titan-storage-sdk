package byterange

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eikenb/pipeat"
	"github.com/quic-go/quic-go/http3"
	"github.com/utopiosphe/titan-storage-sdk/client"
	"github.com/utopiosphe/titan-storage-sdk/request"
)

const (
	minBackoffDelay = 100 * time.Millisecond
	maxBackoffDelay = 3 * time.Second
)

// var log = logging.Logger("range")

type Range struct {
	size       int64
	timeout    time.Duration
	dispatcher *dispatcher
}

func New(size int64, seconds int) *Range {
	if seconds < 1 {
		seconds = 5
	}

	return &Range{
		size:    size,
		timeout: time.Duration(seconds) * time.Second,
	}
}

type Progress struct {
	Written func() int64
	Total   int64
	Done    chan struct{}
}

type ProgressFunc func() Progress

var zeroProgressFunc = func() Progress {
	return Progress{nil, 0, nil}
}

func (r *Range) GetFile(ctx context.Context, resources *client.RangeGetFileReq) (io.ReadCloser, ProgressFunc, error) {

	workerChan, err := r.makeWorkerChan(ctx, resources)
	if err != nil {
		return nil, zeroProgressFunc, err
	}

	fileSize, err := r.getFileSize(ctx, workerChan)
	if err != nil {
		return nil, zeroProgressFunc, err
	}

	reader, writer, err := pipeat.Pipe()
	if err != nil {
		return nil, zeroProgressFunc, err
	}

	d := &dispatcher{
		fileSize:  fileSize,
		rangeSize: r.size,
		reader:    reader,
		writer:    writer,
		workers:   workerChan,
		workloads: newWorkloadIDMapFromMapPointer(resources.Workload),
		resp:      make(chan response, len(workerChan)),
		backoff: &backoff{
			minDelay: minBackoffDelay,
			maxDelay: maxBackoffDelay,
		},
	}
	retProgress := Progress{
		Written: d.writer.GetWrittenBytes,
		Total:   d.fileSize,
		Done:    make(chan struct{}, 1),
	}

	d.run(ctx, retProgress.Done)

	return reader, func() Progress { return retProgress }, nil
}

func (r *Range) GetProgress() float64 {
	if r.dispatcher == nil || r.size == 0 {
		return 0
	}
	return float64(r.dispatcher.writer.GetWrittenBytes()) / float64(r.size)
}

func (r *Range) GetWrittenBytes() int64 {
	if r.dispatcher == nil {
		return 0
	}
	return r.dispatcher.writer.GetWrittenBytes()
}

func (r *Range) getFileSize(ctx context.Context, workerChan chan worker) (int64, error) {
	var (
		start int64 = 0
		size  int64 = 1
	)

	for {
		select {
		case w := <-workerChan:
			req, err := http.NewRequest("GET", w.e, nil)
			if err != nil {
				log.Printf("new request failed: %v", err)
				continue
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+size))

			resp, err := w.c.Do(req)
			if err != nil {
				log.Printf("fetch failed: %v", err)
				continue
			}
			defer func() {
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}
			}()
			v := resp.Header.Get("Content-Range")
			if v != "" {
				subs := strings.Split(v, "/")
				if len(subs) != 2 {
					log.Printf("invalid content range: %s", v)
				}
				return strconv.ParseInt(subs[1], 10, 64)
			}

		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

func (r *Range) makeWorkerChan(ctx context.Context, res *client.RangeGetFileReq) (chan worker, error) {
	workerChan := make(chan worker, len(res.Urls))

	var wg sync.WaitGroup
	wg.Add(len(res.Urls))

	for i, u := range res.Urls {
		go func(idx int) {
			defer wg.Done()

			var tk *client.BodyToken = u.Token
			client := &http.Client{
				Transport: &http3.RoundTripper{TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				}},
				Timeout: 1 * time.Second,
			}

			uu, err := url.Parse(u.Url)
			if err != nil {
				log.Printf("parse url failed: %v", err)
				return
			}

			req := request.Request{
				Jsonrpc: "2.0",
				ID:      "1",
				Method:  "titan.Version",
				Params:  nil,
			}

			rpcUrl := fmt.Sprintf("%s/rpc/v0", uu.Host)
			_, err = request.PostJsonRPC(client, rpcUrl, req, nil)
			if err != nil {
				log.Printf("send packet failed: %v", err)
				return
			}

			client.Timeout = r.timeout

			workerChan <- worker{
				c:          client,
				e:          u.Url,
				tk:         tk,
				nodeID:     u.NodeID,
				workloadID: u.WorkloadID,
			}
		}(i)
	}
	wg.Wait()

	if len(workerChan) == 0 {
		return nil, fmt.Errorf("no worker available")
	}

	return workerChan, nil
}
