package nronthego

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

// payloadEvent defines the structure sent to New Relic.
type payloadEvent struct {
	Timestamp  int64                  `json:"timestamp"` // ms since epoch
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes"`
}

// NewRelicHook is a Logrus hook that asynchronously sends logs to New Relic.
type NewRelicHook struct {
	LicenseKey    string
	AppName       string
	Endpoint      string
	client        *http.Client
	batchSize     int
	flushInterval time.Duration
	maxRetries    int
	backoff       time.Duration
	logCh         chan *logrus.Entry
	quitCh        chan struct{}
	wg            sync.WaitGroup
}

// NewNewRelicHook constructs a hook with sensible defaults.
func NewNewRelicHook(licenseKey, appName string) *NewRelicHook {
	hook := &NewRelicHook{
		LicenseKey:    licenseKey,
		AppName:       appName,
		Endpoint:      "https://log-api.newrelic.com/log/v1",
		client:        &http.Client{Timeout: 5 * time.Second},
		batchSize:     50,
		flushInterval: 1 * time.Second,
		maxRetries:    5,
		backoff:       500 * time.Millisecond,
		logCh:         make(chan *logrus.Entry, 1000),
		quitCh:        make(chan struct{}),
	}
	hook.wg.Add(1)
	go hook.run()
	return hook
}

// Levels returns all log levels for this hook.
func (hook *NewRelicHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire enqueues the log entry for asynchronous delivery.
func (hook *NewRelicHook) Fire(entry *logrus.Entry) error {
	// non-blocking enqueue; drop if full
	select {
	case hook.logCh <- entry:
	default:
		// channel full, drop log
	}
	return nil
}

// run processes batches of log entries and sends them.
func (hook *NewRelicHook) run() {
	defer hook.wg.Done()
	ticker := time.NewTicker(hook.flushInterval)
	defer ticker.Stop()

	batch := make([]*logrus.Entry, 0, hook.batchSize)

	flush := func(entries []*logrus.Entry) {
		if len(entries) == 0 {
			return
		}
		// build payload
		events := make([]payloadEvent, len(entries))
		for i, e := range entries {
			attrs := map[string]interface{}{
				"level":   e.Level.String(),
				"service": hook.AppName,
			}
			// attach user-provided fields
			for k, v := range e.Data {
				attrs[k] = v
			}
			// attach New Relic trace/span if available
			if e.Context != nil {
				if txn := newrelic.FromContext(e.Context); txn != nil {
					md := txn.GetLinkingMetadata()
					attrs["trace.id"] = md.TraceID
					attrs["span.id"] = md.SpanID
				}
			}
			events[i] = payloadEvent{
				Timestamp:  e.Time.UnixNano() / int64(time.Millisecond),
				Message:    e.Message,
				Attributes: attrs,
			}
		}
		data, err := json.Marshal(events)
		if err != nil {
			return
		}

		// retry logic
		var lastErr error
		backoff := hook.backoff
		for attempt := 0; attempt < hook.maxRetries; attempt++ {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, hook.Endpoint, bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-License-Key", hook.LicenseKey)

			resp, err := hook.client.Do(req)
			if err != nil {
				lastErr = err
			} else {
				// drain and close body
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					return
				}
				// handle rate limiting
				if resp.StatusCode == 429 {
					if ra := resp.Header.Get("Retry-After"); ra != "" {
						if secs, err := strconv.Atoi(ra); err == nil {
							time.Sleep(time.Duration(secs) * time.Second)
							continue
						}
					}
					time.Sleep(backoff)
					backoff *= 2
				} else if resp.StatusCode >= 500 {
					time.Sleep(backoff)
					backoff *= 2
				} else {
					// client error, drop
					return
				}
			}
		}
		if lastErr != nil {
			log.Printf("NewRelicHook: failed to send logs: %v", lastErr)
		}
	}

	for {
		select {
		case entry := <-hook.logCh:
			batch = append(batch, entry)
			if len(batch) >= hook.batchSize {
				flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			flush(batch)
			batch = batch[:0]
		case <-hook.quitCh:
			flush(batch)
			return
		}
	}
}

// Close gracefully shuts down the hook, flushing any remaining entries.
func (hook *NewRelicHook) Close() {
	close(hook.quitCh)
	hook.wg.Wait()
}
