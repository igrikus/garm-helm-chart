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

type GithubCredentialsSpec struct {
	Name            string
	Description     string
	Endpoint        string
	AuthType        string
	OAuth2Token     string
	AppID           int64
	InstallationID  int64
	PrivateKeyBytes []byte
}

type GithubCredentialsUpdate struct {
	Description     *string
	OAuth2Token     *string
	AppID           int64
	InstallationID  int64
	PrivateKeyBytes []byte
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

func (c *Client) CreateGithubCredentials(ctx context.Context, in GithubCredentialsSpec) (int64, error) {
	var id int64
	body := garmparams.CreateGithubCredentialsParams{
		Name:        in.Name,
		Description: in.Description,
		Endpoint:    in.Endpoint,
		AuthType:    garmparams.ForgeAuthType(in.AuthType),
	}
	switch body.AuthType {
	case garmparams.ForgeAuthTypeApp:
		body.App = garmparams.GithubApp{
			AppID:           in.AppID,
			InstallationID:  in.InstallationID,
			PrivateKeyBytes: in.PrivateKeyBytes,
		}
	default:
		body.PAT = garmparams.GithubPAT{OAuth2Token: in.OAuth2Token}
	}
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Credentials.CreateCredentials(&credentials.CreateCredentialsParams{
			Context: ctx,
			Body:    body,
		}, auth)
		if err != nil {
			return err
		}
		id = int64(resp.Payload.ID)
		return nil
	})
	return id, err
}

func (c *Client) GetGithubCredentials(ctx context.Context, id int64) (*garmparams.ForgeCredentials, error) {
	var out garmparams.ForgeCredentials
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Credentials.GetCredentials(&credentials.GetCredentialsParams{
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

func (c *Client) UpdateGithubCredentials(ctx context.Context, id int64, in GithubCredentialsUpdate) error {
	body := garmparams.UpdateGithubCredentialsParams{Description: in.Description}
	if in.OAuth2Token != nil {
		body.PAT = &garmparams.GithubPAT{OAuth2Token: *in.OAuth2Token}
	}
	if len(in.PrivateKeyBytes) > 0 || in.AppID != 0 || in.InstallationID != 0 {
		body.App = &garmparams.GithubApp{
			AppID:           in.AppID,
			InstallationID:  in.InstallationID,
			PrivateKeyBytes: in.PrivateKeyBytes,
		}
	}
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Credentials.UpdateCredentials(&credentials.UpdateCredentialsParams{
			Context: ctx,
			ID:      id,
			Body:    body,
		}, auth)
		return err
	})
}

func (c *Client) DeleteGithubCredentials(ctx context.Context, id int64) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Credentials.DeleteCredentials(&credentials.DeleteCredentialsParams{
			Context: ctx,
			ID:      id,
		}, auth)
	})
}
