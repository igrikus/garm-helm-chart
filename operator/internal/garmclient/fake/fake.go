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

type conflict struct{ what string }

func (e *conflict) Error() string { return fmt.Sprintf("%s: conflict", e.what) }
func (e *conflict) Code() int     { return 409 }

type Client struct {
	Endpoints      map[string]garmparams.ForgeEndpoint
	Credentials    map[int64]garmparams.ForgeCredentials
	Orgs           map[string]garmparams.Organization
	Repos          map[string]garmparams.Repository
	Enterprises    map[string]garmparams.Enterprise
	Templates      map[uint]garmparams.Template
	Pools          map[string]garmparams.Pool
	Instances      map[string][]garmparams.Instance // keyed by pool ID
	OrgHooks       map[string]garmparams.HookInfo
	RepoHooks      map[string]garmparams.HookInfo
	ServerSettings garmparams.ControllerInfo
	nextCredID     int64
	nextOrgID      int
	nextRepoID     int
	nextEntID      int
	nextTemplateID uint
	nextPoolID     int
	nextHookID     int64
	CreateOrgFail  bool // toggle for negative tests
}

func New() *Client {
	return &Client{
		Endpoints:   map[string]garmparams.ForgeEndpoint{},
		Credentials: map[int64]garmparams.ForgeCredentials{},
		Orgs:        map[string]garmparams.Organization{},
		Repos:       map[string]garmparams.Repository{},
		Enterprises: map[string]garmparams.Enterprise{},
		Templates:   map[uint]garmparams.Template{},
		Pools:       map[string]garmparams.Pool{},
		Instances:   map[string][]garmparams.Instance{},
		OrgHooks:    map[string]garmparams.HookInfo{},
		RepoHooks:   map[string]garmparams.HookInfo{},
	}
}

func (c *Client) GetServerSettings(_ context.Context) (*garmparams.ControllerInfo, error) {
	out := c.ServerSettings
	return &out, nil
}

func (c *Client) UpdateServerSettings(_ context.Context, in garmclient.ServerSettingsUpdate) error {
	if in.MetadataURL != nil {
		c.ServerSettings.MetadataURL = *in.MetadataURL
	}
	if in.CallbackURL != nil {
		c.ServerSettings.CallbackURL = *in.CallbackURL
	}
	if in.WebhookURL != nil {
		c.ServerSettings.WebhookURL = *in.WebhookURL
	}
	if in.AgentURL != nil {
		c.ServerSettings.AgentURL = *in.AgentURL
	}
	if in.GARMAgentReleasesURL != nil {
		c.ServerSettings.GARMAgentReleasesURL = *in.GARMAgentReleasesURL
	}
	if in.SyncGARMAgentTools != nil {
		c.ServerSettings.SyncGARMAgentTools = *in.SyncGARMAgentTools
	}
	if in.MinimumJobAgeBackoffSeconds != nil {
		c.ServerSettings.MinimumJobAgeBackoff = *in.MinimumJobAgeBackoffSeconds
	}
	if in.ClearCACertBundle {
		c.ServerSettings.CACertBundle = nil
	} else if len(in.CACertBundle) > 0 {
		c.ServerSettings.CACertBundle = in.CACertBundle
	}
	return nil
}

var _ garmclient.Interface = (*Client)(nil)

func (c *Client) CreateGiteaEndpoint(_ context.Context, in garmclient.GiteaEndpointSpec) (string, error) {
	if _, ok := c.Endpoints[in.Name]; ok {
		return "", errors.New("conflict")
	}
	c.Endpoints[in.Name] = garmparams.ForgeEndpoint{
		Name: in.Name, Description: in.Description, APIBaseURL: in.APIBaseURL, BaseURL: in.BaseURL,
		CACertBundle: in.CACertBundle, ToolsMetadataURL: in.ToolsMetadataURL,
		UseInternalToolsMetadata: &in.UseInternalToolsMetadata,
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
	e.ToolsMetadataURL = in.ToolsMetadataURL
	e.UseInternalToolsMetadata = &in.UseInternalToolsMetadata
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

func (c *Client) CreateGithubEndpoint(_ context.Context, in garmclient.GithubEndpointSpec) (string, error) {
	if _, ok := c.Endpoints[in.Name]; ok {
		return "", errors.New("conflict")
	}
	c.Endpoints[in.Name] = garmparams.ForgeEndpoint{
		Name: in.Name, Description: in.Description, APIBaseURL: in.APIBaseURL, UploadBaseURL: in.UploadBaseURL,
		BaseURL: in.BaseURL, CACertBundle: in.CACertBundle, EndpointType: garmparams.GithubEndpointType,
	}
	return in.Name, nil
}

func (c *Client) GetGithubEndpoint(_ context.Context, name string) (*garmparams.ForgeEndpoint, error) {
	e, ok := c.Endpoints[name]
	if !ok {
		return nil, &notFound{what: "endpoint " + name}
	}
	return &e, nil
}

func (c *Client) UpdateGithubEndpoint(_ context.Context, name string, in garmclient.GithubEndpointSpec) error {
	e, ok := c.Endpoints[name]
	if !ok {
		return &notFound{what: "endpoint " + name}
	}
	e.Description = in.Description
	e.APIBaseURL = in.APIBaseURL
	e.UploadBaseURL = in.UploadBaseURL
	e.BaseURL = in.BaseURL
	e.CACertBundle = in.CACertBundle
	c.Endpoints[name] = e
	return nil
}

func (c *Client) DeleteGithubEndpoint(_ context.Context, name string) error {
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

func (c *Client) CreateGithubCredentials(_ context.Context, in garmclient.GithubCredentialsSpec) (int64, error) {
	ep, ok := c.Endpoints[in.Endpoint]
	if !ok {
		return 0, &notFound{what: "endpoint " + in.Endpoint}
	}
	c.nextCredID++
	id := c.nextCredID
	c.Credentials[id] = garmparams.ForgeCredentials{
		ID: uint(id), Name: in.Name, Description: in.Description, BaseURL: ep.BaseURL,
		APIBaseURL: ep.APIBaseURL, UploadBaseURL: ep.UploadBaseURL, ForgeType: garmparams.GithubEndpointType,
		AuthType: garmparams.ForgeAuthType(in.AuthType), Endpoint: ep,
	}
	return id, nil
}

func (c *Client) GetGithubCredentials(_ context.Context, id int64) (*garmparams.ForgeCredentials, error) {
	cr, ok := c.Credentials[id]
	if !ok {
		return nil, &notFound{what: fmt.Sprintf("credentials %d", id)}
	}
	return &cr, nil
}

func (c *Client) UpdateGithubCredentials(_ context.Context, id int64, in garmclient.GithubCredentialsUpdate) error {
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

func (c *Client) DeleteGithubCredentials(_ context.Context, id int64) error {
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
	if in.WebhookSecret == "" {
		return "", errors.New("missing secret")
	}
	for _, org := range c.Orgs {
		if org.Name == in.Name && org.Endpoint.Name == in.EndpointName {
			return "", &conflict{what: "org " + in.Name}
		}
	}
	c.nextOrgID++
	id := fmt.Sprintf("org-%d", c.nextOrgID)
	c.Orgs[id] = garmparams.Organization{
		ID: id, Name: in.Name, CredentialsName: in.CredentialsName,
		WebhookSecret: in.WebhookSecret,
		Endpoint:      garmparams.ForgeEndpoint{Name: in.EndpointName},
	}
	return id, nil
}

func (c *Client) ListOrgs(_ context.Context, name, endpoint string) ([]garmparams.Organization, error) {
	var out []garmparams.Organization
	for _, org := range c.Orgs {
		if name != "" && org.Name != name {
			continue
		}
		if endpoint != "" && org.Endpoint.Name != endpoint {
			continue
		}
		out = append(out, org)
	}
	return out, nil
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
	if in.WebhookSecret != nil {
		o.WebhookSecret = *in.WebhookSecret
	}
	c.Orgs[id] = o
	return nil
}

func (c *Client) DeleteOrg(_ context.Context, id string) error {
	if _, ok := c.Orgs[id]; !ok {
		return &notFound{what: "org " + id}
	}
	delete(c.Orgs, id)
	delete(c.OrgHooks, id)
	return nil
}

func (c *Client) GetOrgWebhookInfo(_ context.Context, id string) (*garmparams.HookInfo, error) {
	h, ok := c.OrgHooks[id]
	if !ok {
		return nil, &notFound{what: "org hook " + id}
	}
	return &h, nil
}

func (c *Client) InstallOrgWebhook(_ context.Context, id string, in garmclient.WebhookInstall) (*garmparams.HookInfo, error) {
	if _, ok := c.Orgs[id]; !ok {
		return nil, &notFound{what: "org " + id}
	}
	if _, ok := c.OrgHooks[id]; ok {
		return nil, errors.New("conflict")
	}
	c.nextHookID++
	h := garmparams.HookInfo{ID: c.nextHookID, URL: "https://garm.example.com/webhooks/controller", Active: true, InsecureSSL: in.InsecureSSL, Events: []string{"workflow_job"}}
	c.OrgHooks[id] = h
	return &h, nil
}

func (c *Client) CreateRepo(_ context.Context, in garmclient.RepoSpec) (string, error) {
	if in.WebhookSecret == "" {
		return "", errors.New("missing secret")
	}
	c.nextRepoID++
	id := fmt.Sprintf("repo-%d", c.nextRepoID)
	c.Repos[id] = garmparams.Repository{
		ID: id, Owner: in.Owner, Name: in.Name, CredentialsName: in.CredentialsName,
		PoolBalancerType: garmparams.PoolBalancerType(in.PoolBalancerType),
		Credentials:      garmparams.ForgeCredentials{Name: in.CredentialsName, ForgeType: garmparams.EndpointType(in.ForgeType)},
		Endpoint:         garmparams.ForgeEndpoint{EndpointType: garmparams.EndpointType(in.ForgeType)},
		WebhookSecret:    in.WebhookSecret,
	}
	return id, nil
}

func (c *Client) GetRepo(_ context.Context, id string) (*garmparams.Repository, error) {
	r, ok := c.Repos[id]
	if !ok {
		return nil, &notFound{what: "repo " + id}
	}
	return &r, nil
}

func (c *Client) UpdateRepo(_ context.Context, id string, in garmclient.EntityUpdate) error {
	r, ok := c.Repos[id]
	if !ok {
		return &notFound{what: "repo " + id}
	}
	if in.CredentialsName != nil {
		r.CredentialsName = *in.CredentialsName
		r.Credentials.Name = *in.CredentialsName
	}
	if in.PoolBalancerType != nil {
		r.PoolBalancerType = garmparams.PoolBalancerType(*in.PoolBalancerType)
	}
	if in.WebhookSecret != nil {
		r.WebhookSecret = *in.WebhookSecret
	}
	c.Repos[id] = r
	return nil
}

func (c *Client) DeleteRepo(_ context.Context, id string, _ bool) error {
	if _, ok := c.Repos[id]; !ok {
		return &notFound{what: "repo " + id}
	}
	delete(c.Repos, id)
	delete(c.RepoHooks, id)
	return nil
}

func (c *Client) GetRepoWebhookInfo(_ context.Context, id string) (*garmparams.HookInfo, error) {
	h, ok := c.RepoHooks[id]
	if !ok {
		return nil, &notFound{what: "repo hook " + id}
	}
	return &h, nil
}

func (c *Client) InstallRepoWebhook(_ context.Context, id string, in garmclient.WebhookInstall) (*garmparams.HookInfo, error) {
	if _, ok := c.Repos[id]; !ok {
		return nil, &notFound{what: "repo " + id}
	}
	if _, ok := c.RepoHooks[id]; ok {
		return nil, errors.New("conflict")
	}
	c.nextHookID++
	h := garmparams.HookInfo{ID: c.nextHookID, URL: "https://garm.example.com/webhooks/controller", Active: true, InsecureSSL: in.InsecureSSL, Events: []string{"workflow_job"}}
	c.RepoHooks[id] = h
	return &h, nil
}

func (c *Client) CreateEnterprise(_ context.Context, in garmclient.EnterpriseSpec) (string, error) {
	if in.WebhookSecret == "" {
		return "", errors.New("missing secret")
	}
	c.nextEntID++
	id := fmt.Sprintf("enterprise-%d", c.nextEntID)
	c.Enterprises[id] = garmparams.Enterprise{
		ID: id, Name: in.Name, CredentialsName: in.CredentialsName,
		PoolBalancerType: garmparams.PoolBalancerType(in.PoolBalancerType),
		Credentials:      garmparams.ForgeCredentials{Name: in.CredentialsName, ForgeType: garmparams.GithubEndpointType},
		Endpoint:         garmparams.ForgeEndpoint{EndpointType: garmparams.GithubEndpointType},
		WebhookSecret:    in.WebhookSecret,
	}
	return id, nil
}

func (c *Client) GetEnterprise(_ context.Context, id string) (*garmparams.Enterprise, error) {
	e, ok := c.Enterprises[id]
	if !ok {
		return nil, &notFound{what: "enterprise " + id}
	}
	return &e, nil
}

func (c *Client) UpdateEnterprise(_ context.Context, id string, in garmclient.EntityUpdate) error {
	e, ok := c.Enterprises[id]
	if !ok {
		return &notFound{what: "enterprise " + id}
	}
	if in.CredentialsName != nil {
		e.CredentialsName = *in.CredentialsName
		e.Credentials.Name = *in.CredentialsName
	}
	if in.PoolBalancerType != nil {
		e.PoolBalancerType = garmparams.PoolBalancerType(*in.PoolBalancerType)
	}
	if in.WebhookSecret != nil {
		e.WebhookSecret = *in.WebhookSecret
	}
	c.Enterprises[id] = e
	return nil
}

func (c *Client) DeleteEnterprise(_ context.Context, id string) error {
	if _, ok := c.Enterprises[id]; !ok {
		return &notFound{what: "enterprise " + id}
	}
	delete(c.Enterprises, id)
	return nil
}

func (c *Client) ListTemplates(_ context.Context, osType, forgeType, partialName string) ([]garmparams.Template, error) {
	var out []garmparams.Template
	for _, t := range c.Templates {
		if osType != "" && string(t.OSType) != osType {
			continue
		}
		if forgeType != "" && string(t.ForgeType) != forgeType {
			continue
		}
		if partialName != "" && t.Name != partialName {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (c *Client) CreateTemplate(_ context.Context, in garmclient.TemplateCreate) (uint, error) {
	for _, t := range c.Templates {
		if t.Name == in.Name && string(t.OSType) == in.OSType && string(t.ForgeType) == in.ForgeType {
			return 0, errors.New("conflict")
		}
	}
	c.nextTemplateID++
	id := c.nextTemplateID
	c.Templates[id] = garmparams.Template{
		ID: id, Name: in.Name, Description: in.Description, Data: append([]byte(nil), in.Data...),
		OSType: commonparams.OSType(in.OSType), ForgeType: garmparams.EndpointType(in.ForgeType),
	}
	return id, nil
}

func (c *Client) GetTemplate(_ context.Context, id uint) (*garmparams.Template, error) {
	t, ok := c.Templates[id]
	if !ok {
		return nil, &notFound{what: fmt.Sprintf("template %d", id)}
	}
	return &t, nil
}

func (c *Client) UpdateTemplate(_ context.Context, id uint, in garmclient.TemplateUpdate) error {
	t, ok := c.Templates[id]
	if !ok {
		return &notFound{what: fmt.Sprintf("template %d", id)}
	}
	if in.Name != nil {
		t.Name = *in.Name
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Data != nil {
		t.Data = append([]byte(nil), in.Data...)
	}
	c.Templates[id] = t
	return nil
}

func (c *Client) DeleteTemplate(_ context.Context, id uint) error {
	if _, ok := c.Templates[id]; !ok {
		return &notFound{what: fmt.Sprintf("template %d", id)}
	}
	delete(c.Templates, id)
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
	id := c.createPool(in)
	p := c.Pools[id]
	p.OrgID = orgID
	c.Pools[id] = p
	return id, nil
}

func (c *Client) createPool(in garmclient.PoolCreate) string {
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
		TemplateID:             in.TemplateID,
		TemplateName:           templateName(c.Templates, in.TemplateID),
		RunnerPrefix:           garmparams.RunnerPrefix{Prefix: in.RunnerPrefix},
	}
	return id
}

func (c *Client) ListOrgPools(_ context.Context, orgID string) ([]garmparams.Pool, error) {
	if _, ok := c.Orgs[orgID]; !ok {
		return nil, &notFound{what: "org " + orgID}
	}
	var out []garmparams.Pool
	for _, p := range c.Pools {
		if p.OrgID == orgID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (c *Client) CreateRepoPool(_ context.Context, repoID string, in garmclient.PoolCreate) (string, error) {
	if _, ok := c.Repos[repoID]; !ok {
		return "", &notFound{what: "repo " + repoID}
	}
	id := c.createPool(in)
	p := c.Pools[id]
	p.RepoID = repoID
	c.Pools[id] = p
	return id, nil
}

func (c *Client) ListRepoPools(_ context.Context, repoID string) ([]garmparams.Pool, error) {
	if _, ok := c.Repos[repoID]; !ok {
		return nil, &notFound{what: "repo " + repoID}
	}
	var out []garmparams.Pool
	for _, p := range c.Pools {
		if p.RepoID == repoID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (c *Client) CreateEnterprisePool(_ context.Context, enterpriseID string, in garmclient.PoolCreate) (string, error) {
	if _, ok := c.Enterprises[enterpriseID]; !ok {
		return "", &notFound{what: "enterprise " + enterpriseID}
	}
	id := c.createPool(in)
	p := c.Pools[id]
	p.EnterpriseID = enterpriseID
	c.Pools[id] = p
	return id, nil
}

func (c *Client) ListEnterprisePools(_ context.Context, enterpriseID string) ([]garmparams.Pool, error) {
	if _, ok := c.Enterprises[enterpriseID]; !ok {
		return nil, &notFound{what: "enterprise " + enterpriseID}
	}
	var out []garmparams.Pool
	for _, p := range c.Pools {
		if p.EnterpriseID == enterpriseID {
			out = append(out, p)
		}
	}
	return out, nil
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
	if in.TemplateID != nil {
		p.TemplateID = *in.TemplateID
		p.TemplateName = templateName(c.Templates, *in.TemplateID)
	}
	c.Pools[id] = p
	return nil
}

func templateName(templates map[uint]garmparams.Template, id uint) string {
	if id == 0 {
		return ""
	}
	return templates[id].Name
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
