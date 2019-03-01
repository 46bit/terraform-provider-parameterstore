package aws

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/hashicorp/terraform/helper/schema"
)

// This data source is based on the `aws_ssm_parameter` data source from the
// AWS Terraform Provider. The difference is that this data source tells SSM
// not to return the decrypted secret and never saves the secret into the
// Terraform schema.

func dataSourceParameter() *schema.Resource {
	return &schema.Resource{
		Read: dataParameterRead,
		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"parameter_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"type": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataParameterRead(d *schema.ResourceData, meta interface{}) error {
	ssmconn := meta.(*AWSClient).ssmconn

	name := d.Get("parameter_name").(string)

	f := false
	paramInput := &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: &f,
	}

	log.Printf("[DEBUG] Reading SSM Parameter: %s", paramInput)
	resp, err := ssmconn.GetParameter(paramInput)

	if err != nil {
		return fmt.Errorf("Error describing SSM parameter: %s", err)
	}

	param := resp.Parameter
	d.SetId(*param.Name)

	arn := arn.ARN{
		Partition: meta.(*AWSClient).partition,
		Region:    meta.(*AWSClient).region,
		Service:   "ssm",
		AccountID: meta.(*AWSClient).accountid,
		Resource:  fmt.Sprintf("parameter/%s", strings.TrimPrefix(d.Id(), "/")),
	}
	d.Set("arn", arn.String())
	d.Set("parameter_name", param.Name)
	d.Set("type", param.Type)

	return nil
}
