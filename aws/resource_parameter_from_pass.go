package aws

import (
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/scrypt"
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
			// We use a cryptographic hash of the decrypted value from `pass` as a fingerprint
			// of whether the secret in `pass` has changed between Terraform runs. This means that
			// a cryptographic hash of the secret value goes into the Terraform state files.
			// scrypt was chosen because it is suitable for storing encrypted passwords and
			// provides a fixed output for a fixed input (unlike bcrypt.)
			"scrypt_of_value_in_pass": {
				Type:     schema.TypeString,
				Optional: true,
			},
			// We use changes to the Last Modified time of the Parameter in Parameter Store
			// to detect changes since the last Terraform run. This was chosen for two reasons:
			//
			// - It does not require decrypting secrets from Parameter Store, which (e.g.)
			//   reduces event noise in CloudTrail;
			// - Encrypted values returned from Parameter Store change constantly and so cannot
			//   be used to fingerprint of the secret's value.
			"last_modified_in_parameterstore": {
				Type:     schema.TypeString,
				Optional: true,
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

	secret, err := fetchSecretFromPass(d.Get("pass_dir").(string), d.Get("pass_key").(string))
	if err != nil {
		return fmt.Errorf("error fetching `pass` secret: %s", err)
	}
	scryptBytes, err := scrypt.GenerateFromPassword([]byte(secret), 10)
	if err != nil {
		return fmt.Errorf("error bcrypting `pass` secret: %s", err)
	}
	d.Set("bcrypt_of_value_in_pass", hex.EncodeToString(bcryptBytes))

	log.Printf("[DEBUG] Reading Metadata of SSM Parameter: %s", d.Id())

	describeParamsInput := &ssm.DescribeParametersInput{
		ParameterFilters: []*ssm.ParameterStringFilter{
			{
				Key:    aws.String("Name"),
				Option: aws.String("Equals"),
				Values: []*string{aws.String(d.Id())},
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
	d.Set("parameter_name", detail.Name)
	d.Set("key_id", detail.KeyId)
	d.Set("description", detail.Description)
	d.Set("last_modified_in_parameterstore", detail.LastModifiedDate.String())

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

	secret, err := fetchSecretFromPass(d.Get("pass_dir").(string), d.Get("pass_key").(string))
	if err != nil {
		return err
	}

	log.Printf("[INFO] Creating SSM Parameter: %s", d.Get("parameter_name").(string))

	paramInput := &ssm.PutParameterInput{
		Name:      aws.String(d.Get("parameter_name").(string)),
		Type:      aws.String("SecureString"),
		Value:     aws.String(secret),
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

func fetchSecretFromPass(passDir, passKey string) (string, error) {
	log.Printf("[DEBUG] Fetching '%s' from `pass` '%s'", passKey, passDir)
	cmd := exec.Command("pass", passKey)
	if len(passDir) > 0 {
		cmd.Env = append(os.Environ(), fmt.Sprintf("PASSWORD_STORE_DIR=%s", passDir))
	}
	output, err := cmd.Output()
	if err != nil {
		// FIXME: That this error is an `*exec.ExitError` is an implementation detail;
		// code this more defensively.
		return "", fmt.Errorf("%s / %s", err, err.(*exec.ExitError).Stderr)
	}
	return string(output), nil
}

func fetchScryptedSecretFromPass(passDir, passKey) (string, error) {
	secret, err := fetchSecretFromPass(passDir, passKey)
	if err != nil {
		return "", err
	}
	scrypt.Key()
}
