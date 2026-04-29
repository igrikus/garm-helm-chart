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

	commonparams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/client/templates"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

type TemplateCreate struct {
	Name        string
	Description string
	OSType      string
	ForgeType   string
	Data        []byte
}

type TemplateUpdate struct {
	Name        *string
	Description *string
	Data        []byte
}

func (c *Client) ListTemplates(ctx context.Context, osType, forgeType, partialName string) ([]garmparams.Template, error) {
	var out []garmparams.Template
	params := &templates.ListTemplatesParams{Context: ctx}
	if osType != "" {
		params.OsType = &osType
	}
	if forgeType != "" {
		params.ForgeType = &forgeType
	}
	if partialName != "" {
		params.PartialName = &partialName
	}
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Templates.ListTemplates(params, auth)
		if err != nil {
			return err
		}
		out = resp.Payload
		return nil
	})
	return out, err
}

func (c *Client) CreateTemplate(ctx context.Context, in TemplateCreate) (uint, error) {
	var id uint
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Templates.CreateTemplate(&templates.CreateTemplateParams{
			Context: ctx,
			Body: garmparams.CreateTemplateParams{
				Name:        in.Name,
				Description: in.Description,
				OSType:      commonparams.OSType(in.OSType),
				ForgeType:   garmparams.EndpointType(in.ForgeType),
				Data:        in.Data,
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

func (c *Client) GetTemplate(ctx context.Context, id uint) (*garmparams.Template, error) {
	var out garmparams.Template
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Templates.GetTemplate(&templates.GetTemplateParams{
			Context:    ctx,
			TemplateID: float64(id),
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

func (c *Client) UpdateTemplate(ctx context.Context, id uint, in TemplateUpdate) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Templates.UpdateTemplate(&templates.UpdateTemplateParams{
			Context:    ctx,
			TemplateID: float64(id),
			Body: garmparams.UpdateTemplateParams{
				Name:        in.Name,
				Description: in.Description,
				Data:        in.Data,
			},
		}, auth)
		return err
	})
}

func (c *Client) DeleteTemplate(ctx context.Context, id uint) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Templates.DeleteTemplate(&templates.DeleteTemplateParams{
			Context:    ctx,
			TemplateID: float64(id),
		}, auth)
	})
}
