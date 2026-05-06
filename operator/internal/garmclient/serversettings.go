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

	"github.com/cloudbase/garm/client/controller"
	controllerinfo "github.com/cloudbase/garm/client/controller_info"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

type ServerSettingsUpdate struct {
	MetadataURL                 *string
	CallbackURL                 *string
	WebhookURL                  *string
	AgentURL                    *string
	GARMAgentReleasesURL        *string
	SyncGARMAgentTools          *bool
	MinimumJobAgeBackoffSeconds *uint
	CACertBundle                []byte
	ClearCACertBundle           bool
}

func (c *Client) GetServerSettings(ctx context.Context) (*garmparams.ControllerInfo, error) {
	var out garmparams.ControllerInfo
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.ControllerInfo.ControllerInfo(&controllerinfo.ControllerInfoParams{
			Context: ctx,
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

func (c *Client) UpdateServerSettings(ctx context.Context, in ServerSettingsUpdate) error {
	clearCABundle := in.ClearCACertBundle
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		_, err := c.api.Controller.UpdateController(&controller.UpdateControllerParams{
			Context: ctx,
			Body: garmparams.UpdateControllerParams{
				MetadataURL:          in.MetadataURL,
				CallbackURL:          in.CallbackURL,
				WebhookURL:           in.WebhookURL,
				AgentURL:             in.AgentURL,
				GARMAgentReleasesURL: in.GARMAgentReleasesURL,
				SyncGARMAgentTools:   in.SyncGARMAgentTools,
				MinimumJobAgeBackoff: in.MinimumJobAgeBackoffSeconds,
				CACertBundle:         in.CACertBundle,
				ClearCACertBundle:    &clearCABundle,
			},
		}, auth)
		return err
	})
}
