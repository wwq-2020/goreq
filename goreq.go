package goreq

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
)

type Transport func(*http.Request) (*http.Response, error)

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req)
}

type Builder interface {
	URL(url string) Builder
	BaseURL(baseURL string) Builder
	Codec(Codec) Builder
	Method(method string) Builder
	Req(req interface{}) Builder
	Resp(resp interface{}) Builder
	QueryString(key string, value string) Builder
	Header(key string, value string) Builder
	Do(ctx context.Context) error
	WrapTransport(transportWrappers ...func(http.RoundTripper) http.RoundTripper) Builder
}

type builder struct {
	baseURL           string
	url               string
	method            string
	codec             Codec
	resp              interface{}
	req               interface{}
	client            *http.Client
	values            url.Values
	header            http.Header
	transportWrappers []func(http.RoundTripper) http.RoundTripper
	rebuildTransport  int32
	cachedTransport   http.RoundTripper
}

func New() Builder {
	return &builder{
		values:           make(url.Values),
		header:           make(http.Header),
		rebuildTransport: 1,
	}
}

func (b *builder) WrapTransport(transportWrappers ...func(http.RoundTripper) http.RoundTripper) Builder {
	b.transportWrappers = append(b.transportWrappers, transportWrappers...)
	return b
}

func (b *builder) URL(url string) Builder {
	b.url = url
	return b
}

func (b *builder) BaseURL(baseURL string) Builder {
	b.baseURL = baseURL
	return b
}

func (b *builder) Method(method string) Builder {
	b.method = method
	return b
}

func (b *builder) QueryString(key string, value string) Builder {
	b.values.Add(key, value)
	return b
}

func (b *builder) Header(key string, value string) Builder {
	b.header.Add(key, value)
	return b
}

func (b *builder) Codec(codec Codec) Builder {
	b.codec = codec
	return b
}

func (b *builder) Resp(resp interface{}) Builder {
	b.resp = resp
	return b
}

func (b *builder) Req(req interface{}) Builder {
	b.req = req
	return b
}

func (b *builder) Client(client *http.Client) Builder {
	b.client = client
	return b
}

func (b *builder) buildURL() string {
	url := b.url
	if b.baseURL != "" {
		url = b.baseURL + b.url
	}
	if len(b.values) != 0 {
		url = url + "?" + b.values.Encode()
	}
	return url
}

func (b *builder) buildMethod() string {
	method := b.method
	if b.method == "" {
		method = http.MethodGet
	}
	return method
}

func (b *builder) buildCodec() Codec {
	var codec Codec = b.codec
	if b.codec == nil {
		codec = defaultCodec
	}
	return codec
}

func (b *builder) buildTransport() http.RoundTripper {
	if atomic.LoadInt32(&b.rebuildTransport) != 1 {
		return b.cachedTransport
	}
	transport := http.DefaultTransport
	for _, transportWrapper := range b.transportWrappers {
		transport = transportWrapper(transport)
	}
	b.cachedTransport = transport
	atomic.StoreInt32(&b.rebuildTransport, 1)
	return transport
}

func (b *builder) buildClient() *http.Client {
	return &http.Client{
		Transport: b.buildTransport(),
	}
}

func (b *builder) Do(ctx context.Context) error {
	url := b.buildURL()
	method := b.buildMethod()
	codec := b.buildCodec()
	var body io.Reader
	if b.req != nil {
		data, err := codec.Encode(b.req)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	httpReq.Header = b.header
	client := b.buildClient()
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if b.resp != nil {
		err := codec.Decode(httpResp.Body, b.resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func URL(url string) Builder {
	return New().URL(url)
}

func BaseURL(baseURL string) Builder {
	return New().BaseURL(baseURL)
}

func Method(method string) Builder {
	return New().Method(method)
}

func Req(req interface{}) Builder {
	return New().Req(req)
}

func Resp(resp interface{}) Builder {
	return New().Resp(resp)
}

func QueryString(key string, value string) Builder {
	return New().QueryString(key, value)
}

func Header(key string, value string) Builder {
	return New().Header(key, value)
}

func WrapTransport(transportWrappers ...func(http.RoundTripper) http.RoundTripper) Builder {
	return New().WrapTransport(transportWrappers...)
}
