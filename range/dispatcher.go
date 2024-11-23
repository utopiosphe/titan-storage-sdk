package byterange

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/eikenb/pipeat"
	"github.com/pkg/errors"
	"github.com/utopiosphe/titan-storage-sdk/client"
)

type dispatcher struct {
	fileSize  int64
	rangeSize int64
	todos     JobQueue
	workers   chan worker
	resp      chan response
	writer    *pipeat.PipeWriterAt
	reader    *pipeat.PipeReaderAt
	backoff   *backoff
}

type worker struct {
	c  *http.Client
	e  string
	tk *client.BodyToken
}

type response struct {
	offset int64
	data   []byte
}

type job struct {
	index int
	start int64
	end   int64
	retry int
}

type backoff struct {
	minDelay time.Duration
	maxDelay time.Duration
}

func (b *backoff) next(attempt int) time.Duration {
	if attempt < 0 {
		return b.minDelay
	}

	minf := float64(b.minDelay)
	durf := minf * math.Pow(1.5, float64(attempt))
	durf = durf + rand.Float64()*minf

	delay := time.Duration(durf)
	if delay > b.maxDelay {
		return b.maxDelay
	}

	return delay
}

func (d *dispatcher) generateJobs() {
	count := int64(math.Ceil(float64(d.fileSize) / float64(d.rangeSize)))
	for i := int64(0); i < count; i++ {
		start := i * d.rangeSize
		end := (i + 1) * d.rangeSize

		if end > d.fileSize {
			end = d.fileSize
		}

		newJob := &job{
			index: int(i),
			start: start,
			end:   end,
		}

		d.todos.Push(newJob)
	}
}

func (d *dispatcher) run(ctx context.Context, sig chan struct{}) {
	d.generateJobs()
	d.writeData(ctx, sig)

	var (
		counter  int64
		finished = make(chan int64, 1)
	)

	go func() {
		for {
			select {
			case w := <-d.workers:
				go func() {
					j, ok := d.todos.Pop()
					if !ok {
						d.workers <- w
						return
					}

					data, err := d.fetch(ctx, w, j)
					if err != nil {
						errMsg := fmt.Sprintf("pull data failed : %v", err)
						if j.retry > 0 {
							log.Printf("pull data failed (retries: %d): %v", j.retry, err)
							<-time.After(d.backoff.next(j.retry))
						}

						log.Println(errMsg)

						j.retry++
						d.todos.PushFront(j)
						d.workers <- w
						return
					}

					dataLen := j.end - j.start

					if int64(len(data)) < dataLen {
						log.Printf("unexpected data size, want %d got %d", dataLen, len(data))
						d.todos.PushFront(j)
						d.workers <- w
						return
					}

					d.workers <- w
					log.Printf("fetched data from %d to %d, currnet count: %d", j.start, j.end, counter)
					d.resp <- response{
						data:   data[:dataLen],
						offset: j.start,
					}
					finished <- dataLen
				}()
			case size := <-finished:
				log.Printf("counter: %d, received: %d, file-size: %d", counter, size, d.fileSize)
				counter += size
				if counter >= d.fileSize {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (d *dispatcher) writeData(ctx context.Context, sig chan struct{}) {
	go func() {
		defer d.finally(sig)

		var count int64
		for {
			select {
			case r := <-d.resp:
				_, err := d.writer.WriteAt(r.data, r.offset)
				if err != nil {
					log.Printf("write data failed: %v", err)
					continue
				}
				// log.Printf("write data success: %d, length: %d", r.offset, len(r.data))
				count += int64(len(r.data))
				if count >= d.fileSize {
					return
				}
			case <-ctx.Done():
				return
			}
		}

	}()
}

// type timeCal struct {
// 	t    time.Time
// 	done chan struct{}
// }

// func (t *timeCal) cal() {
// 	for {
// 		select {
// 		case <-t.done:
// 			log.Printf("cal done")
// 			return
// 		default:
// 			log.Printf("time has passed: %v", time.Since(t.t))
// 		}
// 		<-time.After(1 * time.Second)
// 	}
// }

func (d *dispatcher) fetch(ctx context.Context, w worker, j *job) ([]byte, error) {
	// startTime := time.Now()

	var buf bytes.Buffer
	if w.tk != nil {
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(w.tk); err != nil {
			return nil, errors.Errorf("encode token failed: %v", err)
		}
	}

	req, err := http.NewRequest("GET", w.e, &buf)
	if err != nil {
		return nil, errors.Errorf("new request failed: %v", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", j.start, j.end))
	// resp, err := w.c.Do(req)
	resp, err := w.c.Do(req)
	if err != nil {
		return nil, errors.Errorf("fetch failed: %v", err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download chunk: %d-%d, status code: %d", j.start, j.end, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("read data failed: %v", err)
	}

	// elapsed := time.Since(startTime)
	// log.Printf("Chunk: %fs, Link: %s, Range: %d-%d", elapsed.Seconds(), w.e, j.start, j.end)

	return data, nil
}

func (d *dispatcher) finally(sig chan struct{}) {
	if sig != nil {
		sig <- struct{}{}
	}
	if err := d.writer.Close(); err != nil {
		log.Printf("close write failed: %v", err)
	}
}
