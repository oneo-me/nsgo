//go:build darwin

package apple

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// Image downloads go through URLSession on macOS, iOS and watchOS.
func init() { imageload.HTTP = httpGet }

var urlSession struct {
	sync.Once
	id objc.ID
}

// session is the downloads' URLSession: ephemeral and without a cache,
// as imageload keeps its own under HTTP's rules.
func session() objc.ID {
	urlSession.Do(func() {
		config := objc.ID(Class("NSURLSessionConfiguration")).Send(sel("ephemeralSessionConfiguration"))
		config.Send(sel("setURLCache:"), objc.ID(0))
		config.Send(sel("setRequestCachePolicy:"), uint(1)) // NSURLRequestReloadIgnoringLocalCacheData
		urlSession.id = objc.ID(Class("NSURLSession")).Send(sel("sessionWithConfiguration:"), config).Send(sel("retain"))
	})
	return urlSession.id
}

type httpResult struct {
	resp *imageload.Response
	err  error
}

// httpGet is imageload.HTTP: a data task, whose progress it reads every
// 100 ms, cancelled with ctx.
func httpGet(ctx context.Context, url string, header imageload.Header, progress imageload.Progress) (*imageload.Response, error) {
	done := make(chan httpResult, 1)
	var task objc.ID
	withPool(func() {
		u := objc.ID(Class("NSURL")).Send(sel("URLWithString:"), NSString(url))
		if u == 0 {
			return
		}
		req := objc.ID(Class("NSMutableURLRequest")).Send(sel("requestWithURL:"), u)
		for k, vs := range header {
			for _, v := range vs {
				req.Send(sel("addValue:forHTTPHeaderField:"), NSString(v), NSString(k))
			}
		}
		// On the session's queue: read what the objects hold here, as
		// they are gone once the handler returns.
		handler := objc.NewBlock(func(_ objc.Block, data, response, failure objc.ID) {
			if failure != 0 {
				done <- httpResult{err: errors.New("apple: " + GoString(failure.Send(sel("localizedDescription"))))}
				return
			}
			r := &imageload.Response{Status: objc.Send[int](response, sel("statusCode")), Header: imageload.Header{}, Body: Bytes(data)}
			fields := response.Send(sel("allHeaderFields"))
			keys := fields.Send(sel("allKeys"))
			for i := range objc.Send[uint](keys, sel("count")) {
				k := keys.Send(sel("objectAtIndex:"), i)
				r.Header.Add(GoString(k), GoString(fields.Send(sel("objectForKey:"), k)))
			}
			done <- httpResult{resp: r}
		})
		task = session().Send(sel("dataTaskWithRequest:completionHandler:"), req, handler).Send(sel("retain"))
		handler.Release()
		task.Send(sel("resume"))
	})
	if task == 0 {
		return nil, errors.New("apple: not a URL: " + url)
	}
	defer task.Send(sel("release"))
	var tick <-chan time.Time
	if progress != nil {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		tick = t.C
	}
	for {
		select {
		case r := <-done:
			if r.err == nil && progress != nil {
				progress(int64(len(r.resp.Body)), expected(task))
			}
			return r.resp, r.err
		case <-ctx.Done():
			task.Send(sel("cancel"))
			<-done
			return nil, ctx.Err()
		case <-tick:
			if n := objc.Send[int64](task, sel("countOfBytesReceived")); n > 0 {
				progress(n, expected(task))
			}
		}
	}
}

// expected is the bytes task expects, 0 when unknown.
func expected(task objc.ID) int64 {
	return max(objc.Send[int64](task, sel("countOfBytesExpectedToReceive")), 0)
}
