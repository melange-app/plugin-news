package mailserver

// This file describes two routes that are used throughout the Melange System.

import (
	"airdispat.ch/identity"
	"airdispat.ch/routing"
	"airdispat.ch/tracker"
	"errors"
	"strings"
)

var ServerKey *identity.Identity

func InitRouter() {
	if ServerKey == nil {
		ServerKey, _ = identity.CreateIdentity()
	}
	if RegistrationRouter == nil {
		RegistrationRouter = &tracker.TrackerRouter{
			tracker.GetTrackingServerLocationFromURL("airdispat.ch"),
			ServerKey,
		}
	}
	if LookupRouter == nil {
		LookupRouter = &Router{ServerKey}
	}
}

var RegistrationRouter routing.Router
var LookupRouter routing.Router

type Router struct {
	Origin *identity.Identity
}

func (a *Router) LookupAlias(from string) (*identity.Address, error) {
	comp := strings.Split("@", from)
	if len(comp) != 2 {
		return nil, errors.New("Can't use lookup router without tracker address.")
	}

	url := tracker.GetTrackingServerLocationFromURL(comp[1])
	t := tracker.CreateTrackerListRouterWithStrings(a.Origin, url)

	return t.LookupAlias(from)
}

func (a *Router) Lookup(from string) (*identity.Address, error) {
	return nil, errors.New("Can't use lookup router for non-aliased names.")
}

func (a *Router) Register(key *identity.Identity, alias string) error {
	return errors.New("Can't use LookupRouter for registration.")
}