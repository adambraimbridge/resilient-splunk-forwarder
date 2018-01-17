package main

import (
	"github.com/rcrowley/go-metrics"
	"log"
	"math"
	"sync"
	"time"
)

const (
	sleepTime  = 100
	maxBackoff = 9
	minBackoff = 0
)

type LogRetry interface {
	Enqueue(s string)
}

type LogProcessor interface {
	LogRetry
	Start()
	Stop()
	Dequeue() ([]string, error)
}

type logProcessor struct {
	forwarder  Forwarder
	cache      Cache
	stopped    bool
	inChan     chan string
	outChan    chan string
	wg         sync.WaitGroup
	chanBuffer int
	workers    int
}

func NewLogProcessor(forwarder Forwarder, cache Cache, config appConfig) LogProcessor {
	return &logProcessor{forwarder, cache, false, nil, nil, sync.WaitGroup{}, config.chanBuffer, config.workers}
}

func (logRetry *logProcessor) Start() {
	mutex := sync.Mutex{}
	level := 0
	levelUp := false

	logRetry.outChan = make(chan string, logRetry.chanBuffer)

	for i := 0; i < logRetry.workers; i++ {
		logRetry.wg.Add(1)
		go func() {
			defer logRetry.wg.Done()
			for msg := range logRetry.outChan {
				logRetry.forwarder.forward(msg, func(s string, err error) {
					if err != nil {
						// cache again and retry later
						logRetry.Enqueue(s)
					} else {
					}
					mutex.Lock()
					if err != nil && level < maxBackoff {
						levelUp = true
					}
					mutex.Unlock()
				})
			}
		}()
	}

	logRetry.inChan = make(chan string, logRetry.chanBuffer)
	for i := 0; i < logRetry.workers; i++ {
		logRetry.wg.Add(1)
		go func() {
			defer logRetry.wg.Done()
			for msg := range logRetry.inChan {
				err := logRetry.cache.Put(msg)
				if err != nil {
					log.Printf("Unexpected error when caching messages: %v\n", err)
				}
			}
		}()
	}

	go func() {
		for !logRetry.stopped {
			entries, err := logRetry.Dequeue()
			if err != nil {
				log.Printf("Failure retrieving logs from S3 %v\n", err)
			} else if len(entries) > 0 {
				log.Printf("Read %v messages from S3\n", len(entries))
			}
			for _, entry := range entries {
				mutex.Lock()
				if levelUp {
					if level < maxBackoff {
						level++
					}
					levelUp = false
				} else if level > minBackoff {
					level--
				}
				mutex.Unlock()
				if level > 0 {
					sleepDuration := time.Duration((0.2*math.Pow(2, float64(level))-0.2)*1000) * time.Millisecond

					log.Printf("Sleeping for %v\n", sleepDuration)
					time.Sleep(sleepDuration)
				}

				t := metrics.GetOrRegisterTimer("post.queue.latency", metrics.DefaultRegistry)
				t.Time(func() {
					log.Printf("Sending document to channel")
					logRetry.outChan <- entry
				})
			}

			// don't overwhelm S3 when it's empty
			if len(entries) == 0 {
				time.Sleep(sleepTime * time.Millisecond)
			}
		}
	}()
}

func (logRetry *logProcessor) Stop() {
	logRetry.stopped = true
	close(logRetry.outChan)
	close(logRetry.inChan)
	log.Printf("Waiting buffered channel consumer to finish processing messages\n")
	logRetry.wg.Wait()
}

func (logRetry *logProcessor) Enqueue(s string) {
	logRetry.inChan <- s
}

func (logRetry *logProcessor) Dequeue() ([]string, error) {
	return logRetry.cache.ListAndDelete()
}
