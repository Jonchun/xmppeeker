package xmpp

import (
	"encoding/xml"
)

// An Element represents a single XML Element inside of an XMPP stream. This can be an XMPP stanza, stream management elements, SASL elements, etc.
type Element interface {
	Name() xml.Name // Name() returns the xml.Name of the element.
	XML() string    // XML() returns a string representation of the element's XML.
}

// A GenericElement is used to represent any generic XMPP Element by storing their raw XML as a string as well as their resolved xml.Name.
type GenericElement struct {
	name xml.Name
	xml  string
}

// NewGenericElement creates a GenericElement given a name and the raw XML for an XMPP Element.
func NewGenericElement(name xml.Name, xml string) *GenericElement {
	ge := GenericElement{
		name: name,
		xml:  xml,
	}
	return &ge
}

func (e GenericElement) Name() xml.Name {
	return e.name
}

func (e GenericElement) XML() string {
	return e.xml
}
