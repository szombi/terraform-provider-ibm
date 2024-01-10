// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package kubernetes

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	v1 "github.com/IBM-Cloud/bluemix-go/api/container/containerv1"
	"github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	"github.com/IBM-Cloud/bluemix-go/bmxerror"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/validate"
)

func ResourceIBMContainerALB() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIBMContainerALBCreate,
		ReadContext:   resourceIBMContainerALBRead,
		Update:        resourceIBMContainerALBUpdate,
		Delete:        resourceIBMContainerALBDelete,
		Importer:      &schema.ResourceImporter{},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"alb_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ALB ID",
			},
			"alb_type": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The type of ALB that you want to create.",
			},
			"cluster": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the cluster that the ALB belongs to.",
				ValidateFunc: validate.InvokeValidator(
					"ibm_container_alb",
					"cluster"),
			},
			"enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Computed:    true,
				Description: "If set to true, the ALB is enabled by default.",
			},
			"zone": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The zone where you want to deploy the ALB.",
			},
			"vlan_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The VLAN ID that you want to use for your ALBs.",
			},
			"alb_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The IP address that you want to assign to the ALB.",
			},
			"alb_build": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The version of the ALB you want to use",
			},
			"autoscale": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "autoscale configuration for the ALB",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"max_replicas": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "the maximum ALB replica count",
						},
						"min_replicas": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "the minimum ALB replica count",
						},
						"cpu_average_utilization": {
							Type:        schema.TypeInt,
							Required:    true, //TODO ?optional
							Description: "the average CPU utilization",
							ExactlyOneOf: []string{
								"cpu_average_utilization",
								"custom_metrics",
							},
						},
						"custom_metrics": {
							Type:        schema.TypeString,
							Required:    true, //TODO ?optional
							Description: "custom metric definition for autoscaling",
							ExactlyOneOf: []string{
								"cpu_average_utilization",
								"custom_metrics",
							},
						},
					},
				},
			},
			"health_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The current status of the ALB",
			},
			"resource_group_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ID of the resource group.",
			},
			"user_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "IP assigned by the user",
				Deprecated:  "This field is deprecated",
			},
			"enable": {
				Type:          schema.TypeBool,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{"disable_deployment"},
				Description:   "set to true if ALB needs to be enabled",
				Deprecated:    "This field is deprecated, use enabled instead",
			},
			"disable_deployment": {
				Type:          schema.TypeBool,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{"enable"},
				Description:   "Set to true if ALB needs to be disabled",
				Deprecated:    "This field is deprecated",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "ALB name",
				Deprecated:  "This field is deprecated",
			},
			"replicas": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Desired number of ALB replicas.",
				Deprecated:  "This field is deprecated",
			},
			"resize": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Indicate whether resizing should be done",
				Deprecated:  "This field is deprecated",
			},
			"region": {
				Type:       schema.TypeString,
				Optional:   true,
				Deprecated: "This field is deprecated",
			},
		},
	}
}

func resourceIBMContainerALBCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	albClient, err := meta.(conns.ClientSession).ContainerAPI()
	if err != nil {
		return diag.FromErr(err)
	}
	albAPI := albClient.Albs()

	albClientV2, err := meta.(conns.ClientSession).VpcContainerAPI()
	if err != nil {
		return diag.FromErr(err)
	}
	albAPIV2 := albClientV2.Albs()

	targetEnv, err := getClusterTargetHeader(d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	clusterID := d.Get("cluster").(string)
	vlanID := d.Get("vlan_id").(string)
	zone := d.Get("zone").(string)
	albType := d.Get("alb_type").(string)

	params := v1.CreateALB{
		VlanID: vlanID,
		Type:   albType,
		Zone:   zone,
	}

	if albBuild, ok := d.GetOk("autoscale"); ok {
		err = resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
			clusterDetails, err := albAPI.GetALBUpdatePolicy(clusterID, targetEnv)
			if err != nil {
				retErr := fmt.Errorf("[ERROR] Error getting Alb update policy details %s", err)
				return resource.RetryableError(retErr)
			}
			if clusterDetails.AutoUpdate { //TODO need to test!
				retErr := fmt.Errorf("[ERROR] Alb automatic update is on and alb_version is specified. Please turn off the autoupdate for Alb if you want to specify Alb version.")
				return resource.NonRetryableError(retErr)
			}
			params.IngressImage = albBuild.(string)
			return nil
		})
	}

	if enabled, ok := d.GetOk("enabled"); ok {
		params.EnableByDefault = enabled.(bool)
	}

	if albIP, ok := d.GetOk("alb_ip"); ok {
		params.IP = albIP.(string)
	}

	albResp, err := albAPI.CreateALB(params, clusterID, targetEnv)
	if err != nil {
		return diag.Errorf("[ERROR] Error creating ALb to the cluster %s", err)
	}
	albID := albResp.Alb
	d.SetId(albID)

	if autoscale, ok := d.GetOk("autoscale"); ok {
		data := autoscale.(map[string]interface{})

		autoscaleDetails := containerv2.AutoscaleDetails{
			Config: &containerv2.AutoscaleConfig{},
		}

		if v, ok := data["max_replicas"].(int); ok {
			autoscaleDetails.Config.MaxReplicas = v
		}
		if v, ok := data["min_replicas"].(int); ok {
			autoscaleDetails.Config.MinReplicas = v
		}
		if v, ok := data["cpu_average_utilization"].(int); ok {
			autoscaleDetails.Config.CPUAverageUtilization = v
		}
		if v, ok := data["custom_metrics"].(string); ok {
			autoscaleDetails.Config.CustomMetrics = v
		}

		targetEnvV2, _ := getVpcClusterTargetHeader(d, meta)

		err = resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
			err = albAPIV2.SetALBAutoscaleConfiguration(clusterID, albID, autoscaleDetails, targetEnvV2)
			if err != nil {
				retErr := fmt.Errorf("[ERROR] Failed to set autoscale configuration for ALB %s", err)
				return resource.RetryableError(retErr)
			}
			return nil
		})
	}

	return resourceIBMContainerALBRead(ctx, d, meta)
}

func resourceIBMContainerALBRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	albClient, err := meta.(conns.ClientSession).ContainerAPI()
	if err != nil {
		return diag.FromErr(err)
	}

	albID := d.Id()
	var clusterID string

	albAPI := albClient.Albs()
	targetEnv, err := getAlbTargetHeader(d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	albClientV2, err := meta.(conns.ClientSession).VpcContainerAPI()
	if err != nil {
		return diag.FromErr(err)
	}
	albAPIV2 := albClientV2.Albs()

	targetEnvV2, _ := getVpcClusterTargetHeader(d, meta)

	err = resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
		albConfig, err := albAPI.GetALB(albID, targetEnv)
		if err != nil {
			retErr := fmt.Errorf("[ERROR] Failed to get ALB details %s", err)
			return resource.RetryableError(retErr)
		}
		d.Set("alb_type", albConfig.ALBType)
		d.Set("cluster", albConfig.ClusterID)
		clusterID = albConfig.ClusterID
		d.Set("enabled", albConfig.Enable)
		d.Set("zone", albConfig.Zone)
		d.Set("vlan_id", albConfig.VlanID)
		d.Set("alb_ip", albConfig.ALBIP)
		d.Set("alb_build", albConfig.ALBBuild)
		return nil
	})

	if err != nil {
		return diag.FromErr(err)
	}

	err = resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
		autoscaleDetails, err := albAPIV2.GetALBAutoscaleConfiguration(clusterID, albID, targetEnvV2)
		if err != nil {
			retErr := fmt.Errorf("[ERROR] Failed to set autoscale configuration for ALB %s", err)
			return resource.RetryableError(retErr)
		}
		data := make(map[string]interface{})
		data["max_replicas"] = autoscaleDetails.Config.MaxReplicas
		data["min_replicas"] = autoscaleDetails.Config.MinReplicas
		data["cpu_average_utilization"] = autoscaleDetails.Config.CPUAverageUtilization
		data["custom_metrics"] = autoscaleDetails.Config.CustomMetrics
		d.Set("autoscale", data)
		return nil
	})

	return nil
}

func resourceIBMContainerALBUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	albClient, err := meta.(conns.ClientSession).ContainerAPI()
	if err != nil {
		return diag.FromErr(err)
	}
	albAPI := albClient.Albs()

	targetEnv, err := getAlbTargetHeader(d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	albClientV2, err := meta.(conns.ClientSession).VpcContainerAPI()
	if err != nil {
		return diag.FromErr(err)
	}
	albAPIV2 := albClientV2.Albs()

	targetEnvV2, _ := getVpcClusterTargetHeader(d, meta)

	albID := d.Id()
	/*
	   "alb_id"
	   "alb_type"
	   "cluster"
	   "enabled"
	   "zone"
	   "vlan_id"
	   "alb_ip"
	   "alb_build"
	   "autoscale"
	   	"max_replicas"
	   	"min_replicas"
	   	"cpu_average_utilization"
	   	"custom_metrics"
	*/
	if d.HasChange("enabled") {
		//todo disable or enable
		enabled := d.Get("enabled").(bool)
		if enabled {
			err = resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
				err := albAPI.DisableALB(albID, targetEnv)
				return nil
			})
		} else {

		}
	}
	if d.HasChange("alb_ip") {
		//todo
		//todo enable with new ip
	}
	if d.HasChange("alb_build") {
		// if autoupdate, drop error
		// need to reread
	}
	if d.HasChange("autoscale") {
		// update the autoscale details
	}

	return resourceIBMContainerALBRead(ctx, d, meta)
}

func waitForContainerALB(d *schema.ResourceData, meta interface{}, albID, timeout string, enable, disableDeployment bool) (interface{}, error) {
	albClient, err := meta.(conns.ClientSession).ContainerAPI()
	if err != nil {
		return false, err
	}

	stateConf := &resource.StateChangeConf{
		Pending: []string{"pending"},
		Target:  []string{"active"},
		Refresh: func() (interface{}, string, error) {
			targetEnv, err := getAlbTargetHeader(d, meta)
			if err != nil {
				return nil, "", err
			}
			alb, err := albClient.Albs().GetALB(albID, targetEnv)
			if err != nil {
				if apiErr, ok := err.(bmxerror.RequestFailure); ok && apiErr.StatusCode() == 404 {
					return nil, "", fmt.Errorf("[ERROR] The resource alb %s does not exist anymore: %v", d.Id(), err)
				}
				return nil, "", err
			}
			if enable {
				if !alb.Enable {
					return alb, "pending", nil
				}
			} else if disableDeployment {
				if alb.Enable {
					return alb, "pending", nil
				}
			}
			return alb, "active", nil
		},
		Timeout:    d.Timeout(timeout),
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForState()
}

func resourceIBMContainerALBDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")

	return nil
}

// WaitForWorkerAvailable Waits for worker creation
func waitForClusterAvailable(d *schema.ResourceData, meta interface{}, albID string) (interface{}, error) {
	csClient, err := meta.(conns.ClientSession).ContainerAPI()
	if err != nil {
		return nil, err
	}

	target, err := getAlbTargetHeader(d, meta)
	if err != nil {
		return nil, err
	}

	albConfig, err := csClient.Albs().GetALB(albID, target)
	if err != nil {
		return nil, err
	}

	ClusterID := albConfig.ClusterID

	log.Printf("Waiting for worker of the cluster (%s) wokers to be available.", ClusterID)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"retry", workerProvisioning},
		Target:     []string{workerNormal},
		Refresh:    workerStateRefreshFunc(csClient.Workers(), ClusterID, target),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForState()
}
func getAlbTargetHeader(d *schema.ResourceData, meta interface{}) (v1.ClusterTargetHeader, error) {
	var region string
	if v, ok := d.GetOk("region"); ok {
		region = v.(string)
	}

	sess, err := meta.(conns.ClientSession).BluemixSession()
	if err != nil {
		return v1.ClusterTargetHeader{}, err
	}

	if region == "" {
		region = sess.Config.Region
	}

	targetEnv := v1.ClusterTargetHeader{
		Region: region,
	}

	return targetEnv, nil
}
