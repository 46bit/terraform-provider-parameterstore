package aws

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceParameterFromPass() *schema.Resource {
	return &schema.Resource{
		Create: resourceParameterFromPassPut,
		Read:   resourceParameterFromPassRead,
		Update: resourceParameterFromPassPut,
		Delete: resourceParameterFromPassDelete,
		Exists: resourceParameterFromPassExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"parameter_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"pass_key": {
				Type:     schema.TypeString,
				Required: true,
			},
			"pass_dir": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"arn": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"key_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourceParameterFromPassExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	ssmconn := meta.(*AWSClient).ssmconn
	_, err := ssmconn.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(d.Id()),
		WithDecryption: aws.Bool(false),
	})
	if err != nil {
		if isAWSErr(err, ssm.ErrCodeParameterNotFound, "") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func resourceParameterFromPassRead(d *schema.ResourceData, meta interface{}) error {
	ssmconn := meta.(*AWSClient).ssmconn

	log.Printf("[DEBUG] Reading SSM Parameter: %s", d.Id())

	resp, err := ssmconn.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(d.Id()),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("error getting SSM parameter: %s", err)
	}

	param := resp.Parameter
	d.Set("parameter_name", param.Name)

	describeParamsInput := &ssm.DescribeParametersInput{
		ParameterFilters: []*ssm.ParameterStringFilter{
			{
				Key:    aws.String("Name"),
				Option: aws.String("Equals"),
				Values: []*string{aws.String(d.Get("parameter_name").(string))},
			},
		},
	}
	describeResp, err := ssmconn.DescribeParameters(describeParamsInput)
	if err != nil {
		return fmt.Errorf("error describing SSM parameter: %s", err)
	}

	if describeResp == nil || len(describeResp.Parameters) == 0 || describeResp.Parameters[0] == nil {
		log.Printf("[WARN] SSM Parameter %q not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	detail := describeResp.Parameters[0]
	d.Set("key_id", detail.KeyId)
	d.Set("description", detail.Description)

	arn := arn.ARN{
		Partition: meta.(*AWSClient).partition,
		Region:    meta.(*AWSClient).region,
		Service:   "ssm",
		AccountID: meta.(*AWSClient).accountid,
		Resource:  fmt.Sprintf("parameter/%s", strings.TrimPrefix(d.Id(), "/")),
	}
	d.Set("arn", arn.String())

	return nil
}

func resourceParameterFromPassDelete(d *schema.ResourceData, meta interface{}) error {
	ssmconn := meta.(*AWSClient).ssmconn

	log.Printf("[INFO] Deleting SSM Parameter: %s", d.Id())

	_, err := ssmconn.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(d.Get("parameter_name").(string)),
	})
	if err != nil {
		return fmt.Errorf("error deleting SSM Parameter (%s): %s", d.Id(), err)
	}

	return nil
}

func resourceParameterFromPassPut(d *schema.ResourceData, meta interface{}) error {
	ssmconn := meta.(*AWSClient).ssmconn

	log.Printf("[INFO] Creating SSM Parameter: %s", d.Get("parameter_name").(string))

	passDir := d.Get("pass_dir").(string)
	passKey := d.Get("pass_key").(string)
	log.Printf("[DEBUG] Fetching secret '%s' from Pass directory '%s'", passKey, passDir)
	cmd := exec.Command("pass", passKey)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PASSWORD_STORE_DIR=%s", passDir))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error fetching from pass: %s / %s", err, err.(*exec.ExitError).Stderr)
	}

	paramInput := &ssm.PutParameterInput{
		Name:      aws.String(d.Get("parameter_name").(string)),
		Type:      aws.String("SecureString"),
		Value:     aws.String(string(output)),
		Overwrite: aws.Bool(true),
	}

	if d.HasChange("description") {
		_, n := d.GetChange("description")
		paramInput.Description = aws.String(n.(string))
	}

	if keyID, ok := d.GetOk("key_id"); ok {
		log.Printf("[DEBUG] Setting key_id for SSM Parameter %v: %s", d.Get("parameter_name"), keyID)
		paramInput.SetKeyId(keyID.(string))
	}

	log.Printf("[DEBUG] Waiting for SSM Parameter %v to be updated", d.Get("parameter_name"))
	if _, err := ssmconn.PutParameter(paramInput); err != nil {
		return fmt.Errorf("error creating SSM parameter: %s", err)
	}

	d.SetId(d.Get("parameter_name").(string))

	return resourceParameterFromPassRead(d, meta)
}
