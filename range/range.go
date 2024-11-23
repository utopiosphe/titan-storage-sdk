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
	c          *http.Client
	dispatcher *dispatcher
}

func New(size int64) *Range {
	return &Range{
		size: size,
		c: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
			Timeout:   3 * time.Second,
		},
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

func (r *Range) GetFile(ctx context.Context, resources *client.ShareAssetResult) (io.ReadCloser, ProgressFunc, error) {

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
		resp:      make(chan response, len(workerChan)),
		backoff: &backoff{
			minDelay: minBackoffDelay,
			maxDelay: maxBackoffDelay,
		},
	}
	retProgress := Progress{
		Written: d.writer.GetWrittenBytes,
		Total:   d.fileSize,
		Done:    make(chan struct{}),
	}

	d.run(ctx, retProgress.Done)

	return reader, func() Progress { return retProgress }, nil
}

func (r *Range) GetProgress() float64 {
	if r.dispatcher == nil {
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
			// resp, err := w.c.Do(req)
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
			//"HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\n\r\n400 Bad Requestarset=utf-8\r\n\r\n{\"jsonrpc\":\"2.0\",\"result\":{\"Version\":\"0.1.21+git.5b4fc64+linux-amd64\",\"APIVersion\":65536,\"BlockDelay\":0},\"id\":\"1\"}\n\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00...+3584 more"
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

func (r *Range) makeWorkerChan(ctx context.Context, res *client.ShareAssetResult) (chan worker, error) {
	workerChan := make(chan worker, len(res.URLs))

	var wg sync.WaitGroup
	wg.Add(len(res.URLs))

	for i := range res.URLs {
		go func(idx int) {
			e := res.URLs[idx]

			var tk *client.BodyToken
			if len(res.Token) > 0 && len(res.Token) >= idx {
				tk = res.Token[idx]
			}

			defer wg.Done()

			client := &http.Client{
				Transport: &http3.RoundTripper{TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				}},
				// Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
				Timeout: 10 * time.Second,
			}

			u, err := url.Parse(e)
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

			rpcUrl := fmt.Sprintf("%s/rpc/v0", u.Host)
			_, err = request.PostJsonRPC(client, rpcUrl, req, nil)
			if err != nil {
				log.Printf("send packet failed: %v", err)
				return
			}

			workerChan <- worker{
				c:  client,
				e:  e,
				tk: tk,
			}
		}(i)
	}
	wg.Wait()

	if len(workerChan) == 0 {
		return nil, fmt.Errorf("no worker available")
	}

	return workerChan, nil
}
