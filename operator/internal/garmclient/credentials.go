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

	"github.com/cloudbase/garm/client/credentials"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

// GiteaCredentialsSpec mirrors garm CreateGiteaCredentialsParams but with
// only the fields the operator manages today (PAT auth — App auth is GitHub-only).
type GiteaCredentialsSpec struct {
	Name        string
	Description string
	Endpoint    string
	OAuth2Token string
}

// GiteaCredentialsUpdate carries the mutable subset.
type GiteaCredentialsUpdate struct {
	Description *string
	OAuth2Token *string // when nil, PAT is not rotated
}

func (c *Client) CreateGiteaCredentials(ctx context.Context, in GiteaCredentialsSpec) (int64, error) {
	var id int64
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Credentials.CreateGiteaCredentials(&credentials.CreateGiteaCredentialsParams{
			Context: ctx,
			Body: garmparams.CreateGiteaCredentialsParams{
				Name:        in.Name,
				Description: in.Description,
				Endpoint:    in.Endpoint,
				AuthType:    garmparams.ForgeAuthTypePAT,
				PAT:         garmparams.GithubPAT{OAuth2Token: in.OAuth2Token},
			},
		}, auth)
		if err != nil {
			return err
		}
		id = int64(resp.Payload.ID)
		return nil
	})
	return id, err
}

func (c *Client) GetGiteaCredentials(ctx context.Context, id int64) (*garmparams.ForgeCredentials, error) {
	var out garmparams.ForgeCredentials
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Credentials.GetGiteaCredentials(&credentials.GetGiteaCredentialsParams{
			Context: ctx,
			ID:      id,
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

func (c *Client) UpdateGiteaCredentials(ctx context.Context, id int64, in GiteaCredentialsUpdate) error {
	body := garmparams.UpdateGiteaCredentialsParams{
		Description: in.Description,
	}
	if in.OAuth2Token != nil {
		body.PAT = &garmparams.GithubPAT{OAuth2Token: *in.OAuth2Token}
	}
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Credentials.UpdateGiteaCredentials(&credentials.UpdateGiteaCredentialsParams{
			Context: ctx,
			ID:      id,
			Body:    body,
		}, auth)
		return err
	})
}

func (c *Client) DeleteGiteaCredentials(ctx context.Context, id int64) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Credentials.DeleteGiteaCredentials(&credentials.DeleteGiteaCredentialsParams{
			Context: ctx,
			ID:      id,
		}, auth)
	})
}
