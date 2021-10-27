package goreq

import (
	"encoding/json"
	"io"
)

type Codec interface {
	Decode(io.Reader, interface{}) error
	Encode(interface{}) ([]byte, error)
}

type JsonCodec struct {
}

var defaultCodec = &JsonCodec{}

func (c *JsonCodec) Decode(r io.Reader, obj interface{}) error {
	if err := json.NewDecoder(r).Decode(obj); err != nil {
		return err
	}
	return nil
}

func (c *JsonCodec) Encode(obj interface{}) ([]byte, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return data, nil
}
