package main

import (
	"net"
	"regexp"
	"strings"
)

// https://github.com/asaskevich/govalidator
const (
	DNSName string = `^([a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62}){1}(\.[a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62})*[\._]?$`
)

var rxDNSName = regexp.MustCompile(DNSName)

type vstruct struct{}

var validator = vstruct{}

// IsAddress
func (v vstruct) IsAddress(str string) bool {
	return v.IsIP(str) || v.IsDNSName(str)
}

// IsDNSName will validate the given string as a DNS name
func (v vstruct) IsDNSName(str string) bool {
	if str == "" || len(strings.Replace(str, ".", "", -1)) > 255 {
		// constraints already violated
		return false
	}
	return rxDNSName.MatchString(str)
}

// IsIP checks if a string is either IP version 4 or 6. Alias for `net.ParseIP`
func (v vstruct) IsIP(str string) bool {
	return net.ParseIP(str) != nil
}
