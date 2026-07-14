package otelreceiver

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func protojsonUnmarshal(data []byte, m proto.Message) error {
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, m)
}

func protojsonMarshal(m proto.Message) ([]byte, error) {
	return protojson.Marshal(m)
}
