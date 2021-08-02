package xmpp

import "encoding/xml"

// Matcher.Match returns true if the Element meets the conditions of the Matcher
type Matcher interface {
	Match(Element) bool
}

// NameMatcher is a Matcher that checks to see if e.Name is equal to itself.
type NameMatcher xml.Name

func (m NameMatcher) Match(e Element) bool {
	eName := e.Name()
	if eName.Space == m.Space && eName.Local == m.Local {
		return true
	}
	return false
}

// SpaceMatcher is a Matcher that checks to see if e.Name.Space is equal to itself.
type SpaceMatcher string

func (m SpaceMatcher) Match(e Element) bool {
	return e.Name().Space == string(m)
}

// AllMatcher is a Matcher that matches any Element.
type AllMatcher struct{}

func (m AllMatcher) Match(e Element) bool {
	return true
}
