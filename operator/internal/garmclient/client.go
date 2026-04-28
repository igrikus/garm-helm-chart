/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package garmclient is a thin wrapper over the go-swagger generated client
// at github.com/cloudbase/garm/client. It owns auth (login → JWT bearer,
// refresh on 401) and exposes typed CRUD methods the reconcilers need.
package garmclient

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	apiclient "github.com/cloudbase/garm/client"
	"github.com/cloudbase/garm/client/login"
	garmparams "github.com/cloudbase/garm/params"
)

// Interface is the subset of GARM operations the reconcilers call. Real
// Client and fake.Client both implement it. Methods are added as later
// phases need them; nothing here is unused by current reconcilers.
type Interface interface {
	// Endpoints (Gitea).
	CreateGiteaEndpoint(ctx context.Context, in GiteaEndpointSpec) (string, error)
	GetGiteaEndpoint(ctx context.Context, name string) (*garmparams.ForgeEndpoint, error)
	UpdateGiteaEndpoint(ctx context.Context, name string, in GiteaEndpointSpec) error
	DeleteGiteaEndpoint(ctx context.Context, name string) error

	// Credentials (Gitea).
	CreateGiteaCredentials(ctx context.Context, in GiteaCredentialsSpec) (int64, error)
	GetGiteaCredentials(ctx context.Context, id int64) (*garmparams.ForgeCredentials, error)
	UpdateGiteaCredentials(ctx context.Context, id int64, in GiteaCredentialsUpdate) error
	DeleteGiteaCredentials(ctx context.Context, id int64) error

	// Organizations.
	CreateOrg(ctx context.Context, in OrgSpec) (string, error)
	GetOrg(ctx context.Context, id string) (*garmparams.Organization, error)
	UpdateOrg(ctx context.Context, id string, in OrgUpdate) error
	DeleteOrg(ctx context.Context, id string) error

	// Pools (org-scoped today; repo/enterprise added in Phase 5).
	CreateOrgPool(ctx context.Context, orgID string, in PoolCreate) (string, error)
	GetPool(ctx context.Context, id string) (*garmparams.Pool, error)
	UpdatePool(ctx context.Context, id string, in PoolUpdate) error
	DeletePool(ctx context.Context, id string) error
	ListPoolInstances(ctx context.Context, poolID string) ([]garmparams.Instance, error)

	// Instances.
	DeleteInstance(ctx context.Context, name string, force bool) error
}

// Config sources auth from env. BaseURL is the GARM HTTP root (e.g.
// http://garm.garm.svc:9997). Username/Password authenticate the operator's
// service account on GARM (managed out-of-band; not the same as cluster RBAC).
type Config struct {
	BaseURL  string
	Username string
	Password string
}

// ConfigFromEnv reads GARM_BASE_URL / GARM_USERNAME / GARM_PASSWORD.
// The operator deployment template (Phase 4) mounts these — typically
// GARM_PASSWORD via secretKeyRef.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL:  os.Getenv("GARM_BASE_URL"),
		Username: os.Getenv("GARM_USERNAME"),
		Password: os.Getenv("GARM_PASSWORD"),
	}
	if cfg.BaseURL == "" || cfg.Username == "" || cfg.Password == "" {
		return cfg, errors.New("GARM_BASE_URL, GARM_USERNAME, GARM_PASSWORD must all be set")
	}
	return cfg, nil
}

// Client implements Interface against a real GARM server.
type Client struct {
	cfg Config
	api *apiclient.GarmAPI

	mu    sync.Mutex
	auth  runtime.ClientAuthInfoWriter
	login login.ClientService
}

// New constructs a Client and performs an initial login. Returning here
// (vs. lazy first-call login) surfaces bad creds at startup.
func New(cfg Config) (*Client, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return nil, fmt.Errorf("invalid GARM base URL %q: %w", cfg.BaseURL, err)
	}

	transport := httptransport.New(u.Host, apiclient.DefaultBasePath, []string{u.Scheme})
	c := &Client{
		cfg:   cfg,
		api:   apiclient.New(transport, strfmt.Default),
		login: login.New(transport, strfmt.Default),
	}
	if err := c.refreshAuth(context.Background()); err != nil {
		return nil, fmt.Errorf("garm login: %w", err)
	}
	return c, nil
}

func (c *Client) refreshAuth(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.login.Login(&login.LoginParams{
		Context: ctx,
		Body:    garmparams.PasswordLoginParams{Username: c.cfg.Username, Password: c.cfg.Password},
	}, nil)
	if err != nil {
		return err
	}
	c.auth = httptransport.BearerToken(resp.Payload.Token)
	return nil
}

// authInfo returns the current bearer auth. Read under the lock to avoid
// racing a concurrent refresh.
func (c *Client) authInfo() runtime.ClientAuthInfoWriter {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.auth
}

// call wraps a single GARM operation: invoke fn(authInfo); if it returns 401,
// refresh once and retry. Anything else — including 404, 409 — is returned as-is
// and inspected by typed helpers in errors.go.
func (c *Client) call(ctx context.Context, fn func(runtime.ClientAuthInfoWriter) error) error {
	if err := fn(c.authInfo()); err != nil {
		if !isUnauthorized(err) {
			return err
		}
		if rerr := c.refreshAuth(ctx); rerr != nil {
			return fmt.Errorf("refresh after 401: %w", rerr)
		}
		return fn(c.authInfo())
	}
	return nil
}
