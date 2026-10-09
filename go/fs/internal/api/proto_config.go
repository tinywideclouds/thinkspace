package api

import "google.golang.org/protobuf/encoding/protojson"

var (
	protoMarshaler   = protojson.MarshalOptions{EmitUnpopulated: true}
	protoUnmarshaler = protojson.UnmarshalOptions{DiscardUnknown: true}
)
