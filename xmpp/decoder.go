package xmpp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

const (
	xmlURL      = "http://www.w3.org/XML/1998/namespace"
	xmlnsPrefix = "xmlns"
	xmlPrefix   = "xml"
)

// A Decoder represents an XMPP parser reading a particular input stream. The parser uses xml.Decoder under the hood.
type Decoder struct {
	Header       string
	defaultSpace string
	nsStack      stack
	prefixMap    map[string]string
	reader       io.Reader
	xmlDecoder   *xml.Decoder
}

// NewDecoder creates a new Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	d := Decoder{
		Header:       "",
		defaultSpace: "",
		nsStack:      stack{},
		prefixMap:    make(map[string]string),
		reader:       r,
		xmlDecoder:   xml.NewDecoder(r),
	}
	return &d
}

// NextElement returns the next Element in the stream.
func (d *Decoder) NextElement() (Element, error) {
	buf := new(bytes.Buffer)
	encoder := xml.NewEncoder(buf)
	stopName := xml.Name{}
	for {
		t, err := d.xmlDecoder.RawToken()
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, nil
		}
		switch t1 := t.(type) {
		case xml.StartElement:
			rawTokenCopy := t1.Copy()
			encodeRawToken(encoder, t1)
			// Parse xmlns definitions first
			for _, a := range t1.Attr {
				if a.Name.Space == xmlnsPrefix {
					d.prefixMap[a.Name.Local] = a.Value
				}
				if a.Name.Space == "" && a.Name.Local == xmlnsPrefix {
					d.defaultSpace = a.Value
				}
			}

			// Translate the name of the element
			d.translate(&t1.Name, true)
			d.nsStack.Push(t1.Name.Space)

			// Translate the name of all of the attributes
			for i := range t1.Attr {
				d.translate(&t1.Attr[i].Name, false)
			}

			// Check if this is the start of a stream
			if t1.Name.Space == NSStream && t1.Name.Local == "stream" {
				encoder.Flush()
				stream := NewStream(rawTokenCopy)
				return stream, nil
			}

			// If stopName is empty, we populate it
			if stopName == (xml.Name{}) {
				stopName = t1.Name
			}
		case xml.EndElement:
			encodeRawToken(encoder, t1)
			d.translate(&t1.Name, true)
			d.nsStack.Pop()
			v := d.nsStack.Peek()
			defaultSpace := ""
			if v != nil {
				defaultSpace = v.(string)
			}
			d.defaultSpace = defaultSpace

			// Check if end of stream
			if t1.Name.Space == NSStream && t1.Name.Local == "stream" {
				return StreamEnd{}, nil
			}
			// If the current token is the end element of the start element, we return.
			if stopName == t1.Name {
				encoder.Flush()
				ge := NewGenericElement(t1.Name, buf.String())
				return ge, nil
			}
		case xml.ProcInst:
			// This is to catch and save the XML Header from the raw tokens being processed by the decoder e.g.
			// <?xml version="1.0" encoding="UTF-8"?>
			encodeRawToken(encoder, t1)
			if t1.Target == xmlPrefix {
				encoder.Flush()
				d.Header = buf.String()
				buf.Reset()
			}
		default:
			encodeRawToken(encoder, t1)
		}
	}
}

// translate implements XML name spaces as described by
// https://www.w3.org/TR/REC-xml-names/
// If translate encounters an unrecognized name space prefix,
// it uses the prefix as the Space rather than report an error.
func (d *Decoder) translate(n *xml.Name, isElementName bool) {
	switch {
	case n.Space == xmlnsPrefix:
		return
	case n.Space == "" && !isElementName:
		return
	case n.Space == xmlPrefix:
		n.Space = xmlURL
	case n.Space == "" && n.Local == xmlnsPrefix:
		return
	}
	if v, ok := d.prefixMap[n.Space]; ok {
		n.Space = v
	} else if n.Space == "" {
		n.Space = d.defaultSpace
	}
}

// encodeRawToken is a helper method used to "hack" the standard lib's encoder to ignore namespaces instead of writing a fully custom encoder
func encodeRawToken(e *xml.Encoder, t xml.Token) {
	switch token := t.(type) {
	case xml.StartElement:
		attrs := make([]xml.Attr, 10)
		for _, attr := range token.Attr {
			if attr.Name.Space == xmlnsPrefix {
				// hack to prevent golang's xml encoder from escaping the xmlns attr
				attrCopy := xml.Attr{
					Name: xml.Name{
						Space: "",
						Local: fmt.Sprintf("%s:%s", xmlnsPrefix, attr.Name.Local),
					},
					Value: attr.Value,
				}
				attrs = append(attrs, attrCopy)
			} else {
				attrs = append(attrs, attr)
			}
		}
		token.Attr = attrs

		// hack to prevent golang's xml encoder from rewriting to xmlns attr
		if token.Name.Space != "" {
			token.Name.Local = fmt.Sprintf("%s:%s", token.Name.Space, token.Name.Local)
			token.Name.Space = ""
		}
		e.EncodeToken(token)
	case xml.EndElement:
		// hack to prevent golang's xml encoder from rewriting to xmlns attr
		if token.Name.Space != "" {
			token.Name.Local = fmt.Sprintf("%s:%s", token.Name.Space, token.Name.Local)
			token.Name.Space = ""
		}
		e.EncodeToken(token)
	default:
		e.EncodeToken(token)
	}
}
