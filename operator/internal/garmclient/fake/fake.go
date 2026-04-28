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

	commonparams "github.com/cloudbase/garm-provider-common/params"
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
	Pools         map[string]garmparams.Pool
	Instances     map[string][]garmparams.Instance // keyed by pool ID
	nextCredID    int64
	nextOrgID     int
	nextPoolID    int
	CreateOrgFail bool // toggle for negative tests
}

func New() *Client {
	return &Client{
		Endpoints:   map[string]garmparams.ForgeEndpoint{},
		Credentials: map[int64]garmparams.ForgeCredentials{},
		Orgs:        map[string]garmparams.Organization{},
		Pools:       map[string]garmparams.Pool{},
		Instances:   map[string][]garmparams.Instance{},
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

func tagsFromStrings(in []string) []garmparams.Tag {
	out := make([]garmparams.Tag, 0, len(in))
	for _, t := range in {
		out = append(out, garmparams.Tag{Name: t})
	}
	return out
}

func (c *Client) CreateOrgPool(_ context.Context, orgID string, in garmclient.PoolCreate) (string, error) {
	if _, ok := c.Orgs[orgID]; !ok {
		return "", &notFound{what: "org " + orgID}
	}
	c.nextPoolID++
	id := fmt.Sprintf("pool-%d", c.nextPoolID)
	c.Pools[id] = garmparams.Pool{
		ID:                     id,
		ProviderName:           in.ProviderName,
		MaxRunners:             in.MaxRunners,
		MinIdleRunners:         in.MinIdleRunners,
		Image:                  in.Image,
		Flavor:                 in.Flavor,
		OSType:                 commonparams.OSType(in.OSType),
		OSArch:                 commonparams.OSArch(in.OSArch),
		Tags:                   tagsFromStrings(in.Tags),
		Enabled:                in.Enabled,
		RunnerBootstrapTimeout: in.RunnerBootstrapTimeout,
		ExtraSpecs:             in.ExtraSpecs,
		GitHubRunnerGroup:      in.GitHubRunnerGroup,
		Priority:               in.Priority,
		OrgID:                  orgID,
		RunnerPrefix:           garmparams.RunnerPrefix{Prefix: in.RunnerPrefix},
	}
	return id, nil
}

func (c *Client) GetPool(_ context.Context, id string) (*garmparams.Pool, error) {
	p, ok := c.Pools[id]
	if !ok {
		return nil, &notFound{what: "pool " + id}
	}
	return &p, nil
}

func (c *Client) UpdatePool(_ context.Context, id string, in garmclient.PoolUpdate) error {
	p, ok := c.Pools[id]
	if !ok {
		return &notFound{what: "pool " + id}
	}
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	if in.MaxRunners != nil {
		p.MaxRunners = *in.MaxRunners
	}
	if in.MinIdleRunners != nil {
		p.MinIdleRunners = *in.MinIdleRunners
	}
	if in.RunnerBootstrapTimeout != nil {
		p.RunnerBootstrapTimeout = *in.RunnerBootstrapTimeout
	}
	if in.Image != nil {
		p.Image = *in.Image
	}
	if in.Flavor != nil {
		p.Flavor = *in.Flavor
	}
	if in.OSType != nil {
		p.OSType = commonparams.OSType(*in.OSType)
	}
	if in.OSArch != nil {
		p.OSArch = commonparams.OSArch(*in.OSArch)
	}
	if in.Tags != nil {
		p.Tags = tagsFromStrings(in.Tags)
	}
	if in.ExtraSpecs != nil {
		p.ExtraSpecs = in.ExtraSpecs
	}
	if in.GitHubRunnerGroup != nil {
		p.GitHubRunnerGroup = *in.GitHubRunnerGroup
	}
	if in.RunnerPrefix != nil {
		p.RunnerPrefix = garmparams.RunnerPrefix{Prefix: *in.RunnerPrefix}
	}
	if in.Priority != nil {
		p.Priority = *in.Priority
	}
	c.Pools[id] = p
	return nil
}

func (c *Client) DeletePool(_ context.Context, id string) error {
	if _, ok := c.Pools[id]; !ok {
		return &notFound{what: "pool " + id}
	}
	delete(c.Pools, id)
	delete(c.Instances, id)
	return nil
}

func (c *Client) ListPoolInstances(_ context.Context, poolID string) ([]garmparams.Instance, error) {
	return c.Instances[poolID], nil
}

func (c *Client) DeleteInstance(_ context.Context, name string, _ bool) error {
	for poolID, list := range c.Instances {
		filtered := list[:0]
		removed := false
		for _, i := range list {
			if i.Name == name {
				removed = true
				continue
			}
			filtered = append(filtered, i)
		}
		if removed {
			c.Instances[poolID] = filtered
			return nil
		}
	}
	return &notFound{what: "instance " + name}
}
