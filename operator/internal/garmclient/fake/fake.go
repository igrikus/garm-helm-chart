/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package fake is an in-memory implementation of garmclient.Interface used
// by envtest reconciler specs. It is not safe for concurrent use, which
// matches how envtest drives a single reconciler at a time.
package fake

import (
	"context"
	"errors"
	"fmt"

	garmparams "github.com/cloudbase/garm/params"

	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

type notFound struct{ what string }

func (e *notFound) Error() string { return fmt.Sprintf("%s: not found", e.what) }
func (e *notFound) Code() int     { return 404 }

type Client struct {
	Endpoints     map[string]garmparams.ForgeEndpoint
	Credentials   map[int64]garmparams.ForgeCredentials
	Orgs          map[string]garmparams.Organization
	nextCredID    int64
	nextOrgID     int
	CreateOrgFail bool // toggle for negative tests
}

func New() *Client {
	return &Client{
		Endpoints:   map[string]garmparams.ForgeEndpoint{},
		Credentials: map[int64]garmparams.ForgeCredentials{},
		Orgs:        map[string]garmparams.Organization{},
	}
}

var _ garmclient.Interface = (*Client)(nil)

func (c *Client) CreateGiteaEndpoint(_ context.Context, in garmclient.GiteaEndpointSpec) (string, error) {
	if _, ok := c.Endpoints[in.Name]; ok {
		return "", errors.New("conflict")
	}
	c.Endpoints[in.Name] = garmparams.ForgeEndpoint{
		Name: in.Name, Description: in.Description, APIBaseURL: in.APIBaseURL, BaseURL: in.BaseURL,
		CACertBundle: in.CACertBundle,
	}
	return in.Name, nil
}

func (c *Client) GetGiteaEndpoint(_ context.Context, name string) (*garmparams.ForgeEndpoint, error) {
	e, ok := c.Endpoints[name]
	if !ok {
		return nil, &notFound{what: "endpoint " + name}
	}
	return &e, nil
}

func (c *Client) UpdateGiteaEndpoint(_ context.Context, name string, in garmclient.GiteaEndpointSpec) error {
	e, ok := c.Endpoints[name]
	if !ok {
		return &notFound{what: "endpoint " + name}
	}
	e.Description = in.Description
	e.APIBaseURL = in.APIBaseURL
	e.BaseURL = in.BaseURL
	e.CACertBundle = in.CACertBundle
	c.Endpoints[name] = e
	return nil
}

func (c *Client) DeleteGiteaEndpoint(_ context.Context, name string) error {
	if _, ok := c.Endpoints[name]; !ok {
		return &notFound{what: "endpoint " + name}
	}
	delete(c.Endpoints, name)
	return nil
}

func (c *Client) CreateGiteaCredentials(_ context.Context, in garmclient.GiteaCredentialsSpec) (int64, error) {
	if _, ok := c.Endpoints[in.Endpoint]; !ok {
		return 0, &notFound{what: "endpoint " + in.Endpoint}
	}
	c.nextCredID++
	id := c.nextCredID
	c.Credentials[id] = garmparams.ForgeCredentials{
		ID: uint(id), Name: in.Name, Description: in.Description, BaseURL: c.Endpoints[in.Endpoint].BaseURL,
	}
	return id, nil
}

func (c *Client) GetGiteaCredentials(_ context.Context, id int64) (*garmparams.ForgeCredentials, error) {
	cr, ok := c.Credentials[id]
	if !ok {
		return nil, &notFound{what: fmt.Sprintf("credentials %d", id)}
	}
	return &cr, nil
}

func (c *Client) UpdateGiteaCredentials(_ context.Context, id int64, in garmclient.GiteaCredentialsUpdate) error {
	cr, ok := c.Credentials[id]
	if !ok {
		return &notFound{what: fmt.Sprintf("credentials %d", id)}
	}
	if in.Description != nil {
		cr.Description = *in.Description
	}
	c.Credentials[id] = cr
	return nil
}

func (c *Client) DeleteGiteaCredentials(_ context.Context, id int64) error {
	if _, ok := c.Credentials[id]; !ok {
		return &notFound{what: fmt.Sprintf("credentials %d", id)}
	}
	delete(c.Credentials, id)
	return nil
}

func (c *Client) CreateOrg(_ context.Context, in garmclient.OrgSpec) (string, error) {
	if c.CreateOrgFail {
		return "", errors.New("induced failure")
	}
	c.nextOrgID++
	id := fmt.Sprintf("org-%d", c.nextOrgID)
	c.Orgs[id] = garmparams.Organization{
		ID: id, Name: in.Name, CredentialsName: in.CredentialsName,
	}
	return id, nil
}

func (c *Client) GetOrg(_ context.Context, id string) (*garmparams.Organization, error) {
	o, ok := c.Orgs[id]
	if !ok {
		return nil, &notFound{what: "org " + id}
	}
	return &o, nil
}

func (c *Client) UpdateOrg(_ context.Context, id string, in garmclient.OrgUpdate) error {
	o, ok := c.Orgs[id]
	if !ok {
		return &notFound{what: "org " + id}
	}
	if in.CredentialsName != nil {
		o.CredentialsName = *in.CredentialsName
	}
	c.Orgs[id] = o
	return nil
}

func (c *Client) DeleteOrg(_ context.Context, id string) error {
	if _, ok := c.Orgs[id]; !ok {
		return &notFound{what: "org " + id}
	}
	delete(c.Orgs, id)
	return nil
}
