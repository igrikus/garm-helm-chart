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

	"github.com/cloudbase/garm/client/enterprises"
	"github.com/cloudbase/garm/client/repositories"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

type RepoSpec struct {
	Owner            string
	Name             string
	CredentialsName  string
	WebhookSecret    string
	PoolBalancerType string
	ForgeType        string
}

type EnterpriseSpec struct {
	Name             string
	CredentialsName  string
	WebhookSecret    string
	PoolBalancerType string
}

type EntityUpdate struct {
	CredentialsName  *string
	WebhookSecret    *string
	PoolBalancerType *string
}

func (c *Client) CreateRepo(ctx context.Context, in RepoSpec) (string, error) {
	var id string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Repositories.CreateRepo(&repositories.CreateRepoParams{
			Context: ctx,
			Body: garmparams.CreateRepoParams{
				Owner:            in.Owner,
				Name:             in.Name,
				CredentialsName:  in.CredentialsName,
				WebhookSecret:    in.WebhookSecret,
				PoolBalancerType: garmparams.PoolBalancerType(in.PoolBalancerType),
				ForgeType:        garmparams.EndpointType(in.ForgeType),
			},
		}, auth)
		if err != nil {
			return err
		}
		id = resp.Payload.ID
		return nil
	})
	return id, err
}

func (c *Client) GetRepo(ctx context.Context, id string) (*garmparams.Repository, error) {
	var out garmparams.Repository
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Repositories.GetRepo(&repositories.GetRepoParams{
			Context: ctx,
			RepoID:  id,
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

func (c *Client) UpdateRepo(ctx context.Context, id string, in EntityUpdate) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Repositories.UpdateRepo(&repositories.UpdateRepoParams{
			Context: ctx,
			RepoID:  id,
			Body:    entityUpdateBody(in),
		}, auth)
		return err
	})
}

func (c *Client) DeleteRepo(ctx context.Context, id string, keepWebhook bool) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Repositories.DeleteRepo(&repositories.DeleteRepoParams{
			Context:     ctx,
			RepoID:      id,
			KeepWebhook: &keepWebhook,
		}, auth)
	})
}

func (c *Client) CreateEnterprise(ctx context.Context, in EnterpriseSpec) (string, error) {
	var id string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Enterprises.CreateEnterprise(&enterprises.CreateEnterpriseParams{
			Context: ctx,
			Body: garmparams.CreateEnterpriseParams{
				Name:             in.Name,
				CredentialsName:  in.CredentialsName,
				WebhookSecret:    in.WebhookSecret,
				PoolBalancerType: garmparams.PoolBalancerType(in.PoolBalancerType),
			},
		}, auth)
		if err != nil {
			return err
		}
		id = resp.Payload.ID
		return nil
	})
	return id, err
}

func (c *Client) GetEnterprise(ctx context.Context, id string) (*garmparams.Enterprise, error) {
	var out garmparams.Enterprise
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Enterprises.GetEnterprise(&enterprises.GetEnterpriseParams{
			Context:      ctx,
			EnterpriseID: id,
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

func (c *Client) UpdateEnterprise(ctx context.Context, id string, in EntityUpdate) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Enterprises.UpdateEnterprise(&enterprises.UpdateEnterpriseParams{
			Context:      ctx,
			EnterpriseID: id,
			Body:         entityUpdateBody(in),
		}, auth)
		return err
	})
}

func (c *Client) DeleteEnterprise(ctx context.Context, id string) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Enterprises.DeleteEnterprise(&enterprises.DeleteEnterpriseParams{
			Context:      ctx,
			EnterpriseID: id,
		}, auth)
	})
}

func entityUpdateBody(in EntityUpdate) garmparams.UpdateEntityParams {
	body := garmparams.UpdateEntityParams{}
	if in.CredentialsName != nil {
		body.CredentialsName = *in.CredentialsName
	}
	if in.WebhookSecret != nil {
		body.WebhookSecret = *in.WebhookSecret
	}
	if in.PoolBalancerType != nil {
		body.PoolBalancerType = garmparams.PoolBalancerType(*in.PoolBalancerType)
	}
	return body
}
