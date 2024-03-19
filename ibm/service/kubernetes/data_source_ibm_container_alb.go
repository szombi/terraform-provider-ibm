// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package kubernetes

import (
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DataSourceIBMContainerALB() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceIBMContainerALBRead,

		Schema: map[string]*schema.Schema{
			"alb_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "ALB ID",
			},
			"alb_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ALB type",
			},
			"cluster": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Cluster id",
			},
			"user_ip": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "IP assigned by the user",
			},
			"enable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "set to true if ALB needs to be enabled",
			},
			"zone": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ALB zone",
			},
			"state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ALB state",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status of the ALB",
			},
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The type of Ingress image that you want to use for your ALB deployment.",
			},
			"resource_group_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ID of the resource group.",
			},
			"disable_deployment": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Set to true if ALB needs to be disabled",
				Deprecated:  "This field deprecated and no longer supported",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ALB name",
				Deprecated:  "This field deprecated and no longer supported",
			},
		},
	}
}

func dataSourceIBMContainerALBRead(d *schema.ResourceData, meta interface{}) error {
	albClient, err := meta.(conns.ClientSession).ContainerAPI()
	if err != nil {
		return err
	}

	albID := d.Get("alb_id").(string)

	albAPI := albClient.Albs()
	targetEnv, err := getAlbTargetHeader(d, meta)
	if err != nil {
		return err
	}
	albConfig, err := albAPI.GetALB(albID, targetEnv)
	if err != nil {
		return err
	}

	d.SetId(albID)
	d.Set("alb_type", &albConfig.ALBType)
	d.Set("cluster", &albConfig.ClusterID)
	d.Set("name", &albConfig.Name)
	d.Set("enable", &albConfig.Enable)
	d.Set("disable_deployment", &albConfig.DisableDeployment)
	d.Set("user_ip", &albConfig.ALBIP)
	d.Set("zone", &albConfig.Zone)
	d.Set("state", &albConfig.State)
	d.Set("status", &albConfig.Status)
	d.Set("version", &albConfig.ALBBuild)
	return nil
}
