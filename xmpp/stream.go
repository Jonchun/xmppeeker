package xmpp

import (
	"bytes"
	"encoding/xml"
)

// Stream is a custom Element that represents the start of a stream.
// Unlike normal XMPP Elements, a Stream's XML() method should not return the closing </stream> tag.
// Normal XMPP Elements have a depth=1 whereas the start of a stream is depth=0
type Stream struct {
	From    string
	To      string
	ID      string
	Version string
	rawSE   xml.StartElement
}

// NewStream returns a new Stream that implements xmpp.Element
func NewStream(rawSE xml.StartElement) *Stream {
	stream := Stream{}
	stream.rawSE = rawSE
	for _, attr := range rawSE.Attr {
		switch attr.Name.Local {
		case "id":
			stream.ID = attr.Value
		case "from":
			stream.From = attr.Value
		case "to":
			stream.To = attr.Value
		case "version":
			stream.Version = attr.Value
		}
	}
	return &stream
}

func (s Stream) Name() xml.Name {
	return xml.Name{
		Local: "stream",
		Space: NSStream,
	}
}

func (s Stream) XML() string {
	buf := new(bytes.Buffer)
	encoder := xml.NewEncoder(buf)
	attrs := make([]xml.Attr, 5)
	for _, attr := range s.rawSE.Attr {
		switch attr.Name.Local {
		case "id":
			attr.Value = s.ID
			attrs = append(attrs, attr)
		case "from":
			attr.Value = s.From
			attrs = append(attrs, attr)
		case "to":
			attr.Value = s.To
			attrs = append(attrs, attr)
		case "version":
			attr.Value = s.Version
			attrs = append(attrs, attr)
		default:
			attrs = append(attrs, attr)
		}
	}
	s.rawSE.Attr = attrs
	encodeRawToken(encoder, s.rawSE)
	encoder.Flush()
	return buf.String()
}

// Empty Struct to represent the end of a stream.
type StreamEnd struct{}

func (s StreamEnd) Name() xml.Name {
	return xml.Name{
		Local: "streamend",
		Space: NSStream,
	}
}

func (s StreamEnd) XML() string {
	return "</stream>"
}
