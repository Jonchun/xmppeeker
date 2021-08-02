package xmpp

import "fmt"

// A Router contains Routes which are used to process XMPP Elements via different handlers.
type Router struct {
	routes []Route
}

// NewRouter returns an empty Router
func NewRouter() *Router {
	return &Router{}
}

// AddRoute adds a Route to r.
func (r *Router) AddRoute(route Route) {
	r.AddRoutes(route)
}

// AddRoutes adds a variable number of Routes to r.
func (r *Router) AddRoutes(routes ...Route) {
	r.routes = append(r.routes, routes...)
}

// Route takes an element and executes its relevant Handler if a match is found.
func (r *Router) Route(e Element) error {
	for _, route := range r.routes {
		if route.Match(e) {
			if route.Handler() == nil {
				return fmt.Errorf("found route but handler doesn't exist: %s", e.XML())
			}
			return route.Handler().HandleElement(e)
		}
	}
	return fmt.Errorf("no routes were found that match: %s", e.XML())
}

// A Route is used to check an xmpp.Element and process it via a custom Handler
type Route interface {
	Match(Element) bool
	Handler() Handler
}

// route have a single Handler function that gets called if all of its Matchers return true
type route struct {
	handler  Handler
	matchers []Matcher
}

// NewRoute returns a blank route that matches nothing and has no Handler.
func NewRoute() *route {
	return &route{}
}

func (r *route) SetHandler(h Handler) {
	r.handler = h
}

func (r *route) AddMatcher(matcher Matcher) {
	r.matchers = append(r.matchers, matcher)
}

func (r *route) AddMatchers(matchers ...Matcher) {
	r.matchers = append(r.matchers, matchers...)
}

func (r route) Match(e Element) bool {
	for _, matcher := range r.matchers {
		if matcher.Match(e) {
			return true
		}
	}
	return false
}

func (r route) Handler() Handler {
	return r.handler
}

// Handler is an interface used to handle a route
type Handler interface {
	HandleElement(e Element) error
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as Handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(e Element) error

// HandleElement calls f(s, p)
func (f HandlerFunc) HandleElement(e Element) error {
	return f(e)
}
