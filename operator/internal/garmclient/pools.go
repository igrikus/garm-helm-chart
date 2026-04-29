/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package garmclient

import (
	"context"
	"encoding/json"

	commonparams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/client/enterprises"
	"github.com/cloudbase/garm/client/organizations"
	"github.com/cloudbase/garm/client/pools"
	"github.com/cloudbase/garm/client/repositories"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

// PoolCreate is the wrapper-internal create-pool body. Field names mirror
// garmparams.CreatePoolParams; this indirection keeps go-swagger types out
// of reconciler imports.
type PoolCreate struct {
	ProviderName           string
	MaxRunners             uint
	MinIdleRunners         uint
	Image                  string
	Flavor                 string
	OSType                 string
	OSArch                 string
	Tags                   []string
	Enabled                bool
	RunnerBootstrapTimeout uint
	ExtraSpecs             json.RawMessage
	GitHubRunnerGroup      string
	RunnerPrefix           string
	Priority               uint
}

// PoolUpdate carries pointer fields so unset fields don't overwrite anything
// server-side. Only fields that differ between desired and actual are set
// — that's the no-downtime invariant.
type PoolUpdate struct {
	Enabled                *bool
	MaxRunners             *uint
	MinIdleRunners         *uint
	RunnerBootstrapTimeout *uint
	Image                  *string
	Flavor                 *string
	OSType                 *string
	OSArch                 *string
	Tags                   []string // nil = no change; non-nil (incl. empty) = replace
	ExtraSpecs             json.RawMessage
	GitHubRunnerGroup      *string
	RunnerPrefix           *string
	Priority               *uint
}

// IsEmpty reports whether the update would be a no-op.
func (u PoolUpdate) IsEmpty() bool {
	return u.Enabled == nil &&
		u.MaxRunners == nil &&
		u.MinIdleRunners == nil &&
		u.RunnerBootstrapTimeout == nil &&
		u.Image == nil &&
		u.Flavor == nil &&
		u.OSType == nil &&
		u.OSArch == nil &&
		u.Tags == nil &&
		u.ExtraSpecs == nil &&
		u.GitHubRunnerGroup == nil &&
		u.RunnerPrefix == nil &&
		u.Priority == nil
}

func (c *Client) CreateOrgPool(ctx context.Context, orgID string, in PoolCreate) (string, error) {
	var id string
	body := garmparams.CreatePoolParams{
		ProviderName:           in.ProviderName,
		MaxRunners:             in.MaxRunners,
		MinIdleRunners:         in.MinIdleRunners,
		Image:                  in.Image,
		Flavor:                 in.Flavor,
		OSType:                 commonparams.OSType(in.OSType),
		OSArch:                 commonparams.OSArch(in.OSArch),
		Tags:                   in.Tags,
		Enabled:                in.Enabled,
		RunnerBootstrapTimeout: in.RunnerBootstrapTimeout,
		ExtraSpecs:             in.ExtraSpecs,
		GitHubRunnerGroup:      in.GitHubRunnerGroup,
		Priority:               in.Priority,
	}
	body.RunnerPrefix = garmparams.RunnerPrefix{Prefix: in.RunnerPrefix}
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Organizations.CreateOrgPool(&organizations.CreateOrgPoolParams{
			Context: ctx,
			OrgID:   orgID,
			Body:    body,
		}, auth)
		if err != nil {
			return err
		}
		id = resp.Payload.ID
		return nil
	})
	return id, err
}

func (c *Client) ListOrgPools(ctx context.Context, orgID string) ([]garmparams.Pool, error) {
	var out []garmparams.Pool
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Organizations.ListOrgPools(&organizations.ListOrgPoolsParams{
			Context: ctx,
			OrgID:   orgID,
		}, auth)
		if err != nil {
			return err
		}
		out = resp.Payload
		return nil
	})
	return out, err
}

func (c *Client) CreateRepoPool(ctx context.Context, repoID string, in PoolCreate) (string, error) {
	var id string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Repositories.CreateRepoPool(&repositories.CreateRepoPoolParams{
			Context: ctx,
			RepoID:  repoID,
			Body:    poolCreateBody(in),
		}, auth)
		if err != nil {
			return err
		}
		id = resp.Payload.ID
		return nil
	})
	return id, err
}

func (c *Client) ListRepoPools(ctx context.Context, repoID string) ([]garmparams.Pool, error) {
	var out []garmparams.Pool
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Repositories.ListRepoPools(&repositories.ListRepoPoolsParams{
			Context: ctx,
			RepoID:  repoID,
		}, auth)
		if err != nil {
			return err
		}
		out = resp.Payload
		return nil
	})
	return out, err
}

func (c *Client) CreateEnterprisePool(ctx context.Context, enterpriseID string, in PoolCreate) (string, error) {
	var id string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Enterprises.CreateEnterprisePool(&enterprises.CreateEnterprisePoolParams{
			Context:      ctx,
			EnterpriseID: enterpriseID,
			Body:         poolCreateBody(in),
		}, auth)
		if err != nil {
			return err
		}
		id = resp.Payload.ID
		return nil
	})
	return id, err
}

func (c *Client) ListEnterprisePools(ctx context.Context, enterpriseID string) ([]garmparams.Pool, error) {
	var out []garmparams.Pool
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Enterprises.ListEnterprisePools(&enterprises.ListEnterprisePoolsParams{
			Context:      ctx,
			EnterpriseID: enterpriseID,
		}, auth)
		if err != nil {
			return err
		}
		out = resp.Payload
		return nil
	})
	return out, err
}

func (c *Client) GetPool(ctx context.Context, id string) (*garmparams.Pool, error) {
	var out garmparams.Pool
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Pools.GetPool(&pools.GetPoolParams{
			Context: ctx,
			PoolID:  id,
		}, auth)
		if err != nil {
			return err
		}
		out = resp.Payload
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func poolCreateBody(in PoolCreate) garmparams.CreatePoolParams {
	body := garmparams.CreatePoolParams{
		ProviderName:           in.ProviderName,
		MaxRunners:             in.MaxRunners,
		MinIdleRunners:         in.MinIdleRunners,
		Image:                  in.Image,
		Flavor:                 in.Flavor,
		OSType:                 commonparams.OSType(in.OSType),
		OSArch:                 commonparams.OSArch(in.OSArch),
		Tags:                   in.Tags,
		Enabled:                in.Enabled,
		RunnerBootstrapTimeout: in.RunnerBootstrapTimeout,
		ExtraSpecs:             in.ExtraSpecs,
		GitHubRunnerGroup:      in.GitHubRunnerGroup,
		Priority:               in.Priority,
	}
	body.RunnerPrefix = garmparams.RunnerPrefix{Prefix: in.RunnerPrefix}
	return body
}

func (c *Client) UpdatePool(ctx context.Context, id string, in PoolUpdate) error {
	body := garmparams.UpdatePoolParams{
		Enabled:                in.Enabled,
		MaxRunners:             in.MaxRunners,
		MinIdleRunners:         in.MinIdleRunners,
		RunnerBootstrapTimeout: in.RunnerBootstrapTimeout,
		ExtraSpecs:             in.ExtraSpecs,
		GitHubRunnerGroup:      in.GitHubRunnerGroup,
		Priority:               in.Priority,
		Tags:                   in.Tags,
	}
	if in.Image != nil {
		body.Image = *in.Image
	}
	if in.Flavor != nil {
		body.Flavor = *in.Flavor
	}
	if in.OSType != nil {
		body.OSType = commonparams.OSType(*in.OSType)
	}
	if in.OSArch != nil {
		body.OSArch = commonparams.OSArch(*in.OSArch)
	}
	if in.RunnerPrefix != nil {
		body.RunnerPrefix = garmparams.RunnerPrefix{Prefix: *in.RunnerPrefix}
	}
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Pools.UpdatePool(&pools.UpdatePoolParams{
			Context: ctx,
			PoolID:  id,
			Body:    body,
		}, auth)
		return err
	})
}

func (c *Client) DeletePool(ctx context.Context, id string) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Pools.DeletePool(&pools.DeletePoolParams{
			Context: ctx,
			PoolID:  id,
		}, auth)
	})
}
