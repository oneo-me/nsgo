//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// Image downloads go through WinHTTP.
func init() { imageload.HTTP = httpGet }

var (
	winhttp                = syscall.NewLazyDLL("winhttp.dll")
	winHTTPOpen            = winhttp.NewProc("WinHttpOpen")
	winHTTPSetOption       = winhttp.NewProc("WinHttpSetOption")
	winHTTPConnect         = winhttp.NewProc("WinHttpConnect")
	winHTTPOpenRequest     = winhttp.NewProc("WinHttpOpenRequest")
	winHTTPSendRequest     = winhttp.NewProc("WinHttpSendRequest")
	winHTTPReceiveResponse = winhttp.NewProc("WinHttpReceiveResponse")
	winHTTPQueryHeaders    = winhttp.NewProc("WinHttpQueryHeaders")
	winHTTPReadData        = winhttp.NewProc("WinHttpReadData")
	winHTTPCloseHandle     = winhttp.NewProc("WinHttpCloseHandle")
)

const (
	winHTTPAutomaticProxy   = 4          // WINHTTP_ACCESS_TYPE_AUTOMATIC_PROXY
	winHTTPOptDecompression = 118        // WINHTTP_OPTION_DECOMPRESSION
	winHTTPOptProtocols     = 133        // WINHTTP_OPTION_ENABLE_HTTP_PROTOCOL
	winHTTPFlagSecure       = 0x00800000 // WINHTTP_FLAG_SECURE
	winHTTPStatusCode       = 19         // WINHTTP_QUERY_STATUS_CODE
	winHTTPRawHeaders       = 22         // WINHTTP_QUERY_RAW_HEADERS_CRLF
	winHTTPQueryFlagNumber  = 0x20000000 // WINHTTP_QUERY_FLAG_NUMBER
	winHTTPAllHeaders       = 0xffffffff // (DWORD)-1: the headers' length up to their NUL
	errInsufficientBuffer   = syscall.Errno(122)
)

var winHTTPSession struct {
	sync.Once
	h   uintptr
	err error
}

// session opens the downloads' WinHTTP session once: synchronous, with
// the system's proxies, gzip and HTTP/2 where Windows has them.
func session() (uintptr, error) {
	winHTTPSession.Do(func() {
		h, _, err := winHTTPOpen.Call(uintptr(unsafe.Pointer(wide("NSGo"))), winHTTPAutomaticProxy, 0, 0, 0)
		if h == 0 {
			winHTTPSession.err = fmt.Errorf("windows: WinHttpOpen: %w", err)
			return
		}
		all := uint32(3) // WINHTTP_DECOMPRESSION_FLAG_ALL
		winHTTPSetOption.Call(h, winHTTPOptDecompression, uintptr(unsafe.Pointer(&all)), 4)
		http2 := uint32(1) // WINHTTP_PROTOCOL_FLAG_HTTP2
		winHTTPSetOption.Call(h, winHTTPOptProtocols, uintptr(unsafe.Pointer(&http2)), 4)
		winHTTPSession.h = h
	})
	return winHTTPSession.h, winHTTPSession.err
}

// httpGet is imageload.HTTP: a synchronous request, which closing its
// handle cancels when ctx ends.
func httpGet(ctx context.Context, rawURL string, header imageload.Header, progress imageload.Progress) (*imageload.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	s, err := session()
	if err != nil {
		return nil, err
	}
	port, _ := strconv.Atoi(u.Port()) // 0: the scheme's
	conn, _, err := winHTTPConnect.Call(s, uintptr(unsafe.Pointer(wide(u.Hostname()))), uintptr(port), 0)
	if conn == 0 {
		return nil, fmt.Errorf("windows: WinHttpConnect: %w", err)
	}
	defer winHTTPCloseHandle.Call(conn)
	var flags uintptr
	if u.Scheme == "https" {
		flags = winHTTPFlagSecure
	}
	req, _, err := winHTTPOpenRequest.Call(conn, uintptr(unsafe.Pointer(wide("GET"))),
		uintptr(unsafe.Pointer(wide(u.RequestURI()))), 0, 0, 0, flags)
	if req == 0 {
		return nil, fmt.Errorf("windows: WinHttpOpenRequest: %w", err)
	}
	var closeOnce sync.Once
	closeReq := func() { closeOnce.Do(func() { winHTTPCloseHandle.Call(req) }) }
	defer closeReq()
	defer context.AfterFunc(ctx, closeReq)()
	resp, err := receive(req, header, progress)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return resp, err
}

func receive(req uintptr, header imageload.Header, progress imageload.Progress) (*imageload.Response, error) {
	var lines strings.Builder
	for k, vs := range header {
		for _, v := range vs {
			lines.WriteString(k + ": " + v + "\r\n")
		}
	}
	var headers, headersLen uintptr
	if lines.Len() > 0 {
		headers, headersLen = uintptr(unsafe.Pointer(wide(lines.String()))), winHTTPAllHeaders
	}
	if r, _, err := winHTTPSendRequest.Call(req, headers, headersLen, 0, 0, 0, 0); r == 0 {
		return nil, fmt.Errorf("windows: WinHttpSendRequest: %w", err)
	}
	if r, _, err := winHTTPReceiveResponse.Call(req, 0); r == 0 {
		return nil, fmt.Errorf("windows: WinHttpReceiveResponse: %w", err)
	}
	var status, size uint32 = 0, 4
	if r, _, err := winHTTPQueryHeaders.Call(req, winHTTPStatusCode|winHTTPQueryFlagNumber, 0,
		uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&size)), 0); r == 0 {
		return nil, fmt.Errorf("windows: WinHttpQueryHeaders: %w", err)
	}
	resp := &imageload.Response{Status: int(status), Header: imageload.Header{}}
	raw, err := rawHeaders(req)
	if err != nil {
		return nil, err
	}
	// The first line is the status line.
	for _, line := range strings.Split(raw, "\r\n")[1:] {
		if k, v, ok := strings.Cut(line, ":"); ok {
			resp.Header.Add(k, strings.TrimSpace(v))
		}
	}
	length, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	resp.Body, err = imageload.ReadAll(httpBody(req), length, progress)
	return resp, err
}

// rawHeaders returns the response's headers as WinHTTP received them,
// a line each.
func rawHeaders(req uintptr) (string, error) {
	var size uint32
	r, _, err := winHTTPQueryHeaders.Call(req, winHTTPRawHeaders, 0, 0, uintptr(unsafe.Pointer(&size)), 0)
	if r == 0 && !errors.Is(err, errInsufficientBuffer) {
		return "", fmt.Errorf("windows: WinHttpQueryHeaders: %w", err)
	}
	buf := make([]uint16, size/2+1)
	if r, _, err := winHTTPQueryHeaders.Call(req, winHTTPRawHeaders, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0); r == 0 {
		return "", fmt.Errorf("windows: WinHttpQueryHeaders: %w", err)
	}
	return syscall.UTF16ToString(buf[:size/2]), nil
}

// httpBody reads a request's body with WinHttpReadData.
type httpBody uintptr

func (h httpBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	var n uint32
	if r, _, err := winHTTPReadData.Call(uintptr(h), uintptr(unsafe.Pointer(&p[0])), uintptr(min(len(p), 1<<30)),
		uintptr(unsafe.Pointer(&n))); r == 0 {
		return 0, fmt.Errorf("windows: WinHttpReadData: %w", err)
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}
