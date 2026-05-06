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

	"github.com/cloudbase/garm/client/instances"
	garmparams "github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

func (c *Client) ListPoolInstances(ctx context.Context, poolID string) ([]garmparams.Instance, error) {
	var out []garmparams.Instance
	err := c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		resp, err := c.api.Instances.ListPoolInstances(&instances.ListPoolInstancesParams{
			Context: ctx,
			PoolID:  poolID,
		}, auth)
		if err != nil {
			return err
		}
		out = resp.Payload
		return nil
	})
	return out, err
}

func (c *Client) DeleteInstance(ctx context.Context, name string, force bool) error {
	return c.call(ctx, func(auth runtime.ClientAuthInfoWriter) error {
		return c.api.Instances.DeleteInstance(&instances.DeleteInstanceParams{
			Context:      ctx,
			InstanceName: name,
			ForceRemove:  &force,
		}, auth)
	})
}
