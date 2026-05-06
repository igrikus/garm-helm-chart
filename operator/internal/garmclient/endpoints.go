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

	"github.com/cloudbase/garm/client/endpoints"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

// GiteaEndpointSpec is the wrapper-internal request shape, decoupled from
// generated parameter structs so reconcilers don't import go-swagger types.
type GiteaEndpointSpec struct {
	Name                     string
	Description              string
	APIBaseURL               string
	BaseURL                  string
	CACertBundle             []byte
	ToolsMetadataURL         string
	UseInternalToolsMetadata bool
}

type GithubEndpointSpec struct {
	Name          string
	Description   string
	APIBaseURL    string
	UploadBaseURL string
	BaseURL       string
	CACertBundle  []byte
}

func (c *Client) CreateGiteaEndpoint(ctx context.Context, in GiteaEndpointSpec) (string, error) {
	var name string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Endpoints.CreateGiteaEndpoint(&endpoints.CreateGiteaEndpointParams{
			Context: ctx,
			Body: garmparams.CreateGiteaEndpointParams{
				Name:                     in.Name,
				Description:              in.Description,
				APIBaseURL:               in.APIBaseURL,
				BaseURL:                  in.BaseURL,
				CACertBundle:             in.CACertBundle,
				ToolsMetadataURL:         in.ToolsMetadataURL,
				UseInternalToolsMetadata: &in.UseInternalToolsMetadata,
			},
		}, auth)
		if err != nil {
			return err
		}
		name = resp.Payload.Name
		return nil
	})
	return name, err
}

func (c *Client) GetGiteaEndpoint(ctx context.Context, name string) (*garmparams.ForgeEndpoint, error) {
	var out garmparams.ForgeEndpoint
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Endpoints.GetGiteaEndpoint(&endpoints.GetGiteaEndpointParams{
			Context: ctx,
			Name:    name,
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

func (c *Client) UpdateGiteaEndpoint(ctx context.Context, name string, in GiteaEndpointSpec) error {
	desc := in.Description
	apiURL := in.APIBaseURL
	baseURL := in.BaseURL
	useInternalToolsMetadata := in.UseInternalToolsMetadata
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Endpoints.UpdateGiteaEndpoint(&endpoints.UpdateGiteaEndpointParams{
			Context: ctx,
			Name:    name,
			Body: garmparams.UpdateGiteaEndpointParams{
				Description:              &desc,
				APIBaseURL:               &apiURL,
				BaseURL:                  &baseURL,
				CACertBundle:             in.CACertBundle,
				ToolsMetadataURL:         in.ToolsMetadataURL,
				UseInternalToolsMetadata: &useInternalToolsMetadata,
			},
		}, auth)
		return err
	})
}

func (c *Client) DeleteGiteaEndpoint(ctx context.Context, name string) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Endpoints.DeleteGiteaEndpoint(&endpoints.DeleteGiteaEndpointParams{
			Context: ctx,
			Name:    name,
		}, auth)
	})
}

func (c *Client) CreateGithubEndpoint(ctx context.Context, in GithubEndpointSpec) (string, error) {
	var name string
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Endpoints.CreateGithubEndpoint(&endpoints.CreateGithubEndpointParams{
			Context: ctx,
			Body: garmparams.CreateGithubEndpointParams{
				Name:          in.Name,
				Description:   in.Description,
				APIBaseURL:    in.APIBaseURL,
				UploadBaseURL: in.UploadBaseURL,
				BaseURL:       in.BaseURL,
				CACertBundle:  in.CACertBundle,
			},
		}, auth)
		if err != nil {
			return err
		}
		name = resp.Payload.Name
		return nil
	})
	return name, err
}

func (c *Client) GetGithubEndpoint(ctx context.Context, name string) (*garmparams.ForgeEndpoint, error) {
	var out garmparams.ForgeEndpoint
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Endpoints.GetGithubEndpoint(&endpoints.GetGithubEndpointParams{
			Context: ctx,
			Name:    name,
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

func (c *Client) UpdateGithubEndpoint(ctx context.Context, name string, in GithubEndpointSpec) error {
	desc := in.Description
	apiURL := in.APIBaseURL
	uploadURL := in.UploadBaseURL
	baseURL := in.BaseURL
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Endpoints.UpdateGithubEndpoint(&endpoints.UpdateGithubEndpointParams{
			Context: ctx,
			Name:    name,
			Body: garmparams.UpdateGithubEndpointParams{
				Description:   &desc,
				APIBaseURL:    &apiURL,
				UploadBaseURL: &uploadURL,
				BaseURL:       &baseURL,
				CACertBundle:  in.CACertBundle,
			},
		}, auth)
		return err
	})
}

func (c *Client) DeleteGithubEndpoint(ctx context.Context, name string) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Endpoints.DeleteGithubEndpoint(&endpoints.DeleteGithubEndpointParams{
			Context: ctx,
			Name:    name,
		}, auth)
	})
}
