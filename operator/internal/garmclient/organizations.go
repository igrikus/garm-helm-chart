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

	"github.com/cloudbase/garm/client/organizations"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

// OrgSpec is forge-agnostic at the wire level: GARM disambiguates by ForgeType.
type OrgSpec struct {
	Name             string
	CredentialsName  string
	WebhookSecret    string
	PoolBalancerType string // "" = roundrobin (server default)
	ForgeType        string // "gitea" | "github"
}

type OrgUpdate struct {
	CredentialsName  *string
	WebhookSecret    *string
	PoolBalancerType *string
}

func (c *Client) CreateOrg(ctx context.Context, in OrgSpec) (string, error) {
	var id string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Organizations.CreateOrg(&organizations.CreateOrgParams{
			Context: ctx,
			Body: garmparams.CreateOrgParams{
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

func (c *Client) GetOrg(ctx context.Context, id string) (*garmparams.Organization, error) {
	var out garmparams.Organization
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Organizations.GetOrg(&organizations.GetOrgParams{
			Context: ctx,
			OrgID:   id,
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

func (c *Client) UpdateOrg(ctx context.Context, id string, in OrgUpdate) error {
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
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Organizations.UpdateOrg(&organizations.UpdateOrgParams{
			Context: ctx,
			OrgID:   id,
			Body:    body,
		}, auth)
		return err
	})
}

func (c *Client) DeleteOrg(ctx context.Context, id string) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Organizations.DeleteOrg(&organizations.DeleteOrgParams{
			Context: ctx,
			OrgID:   id,
		}, auth)
	})
}
