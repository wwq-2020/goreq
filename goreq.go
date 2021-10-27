package goreq

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

type Builder interface {
	URL(string) Builder
	Codec(Codec) Builder
	Method(string) Builder
	Req(interface{}) Builder
	Resp(interface{}) Builder
	Do(ctx context.Context) error
}

type builder struct {
	url    string
	method string
	codec  Codec
	resp   interface{}
	req    interface{}
	client *http.Client
	values url.Values
	header http.Header
}

func New() Builder {
	return &builder{
		values: make(url.Values),
		header: make(http.Header),
	}
}

func (b *builder) URL(url string) Builder {
	b.url = url
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

func (b *builder) Do(ctx context.Context) error {
	url := b.url
	if len(b.values) != 0 {
		url = url + "?" + b.values.Encode()
	}
	method := b.method
	if b.method == "" {
		method = http.MethodGet
	}

	var codec Codec = b.codec
	if b.codec == nil {
		codec = defaultCodec
	}
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
	client := b.client
	if client == nil {
		client = http.DefaultClient
	}

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
