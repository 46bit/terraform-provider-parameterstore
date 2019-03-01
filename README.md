`passrameter-store` Terraform Provider
======================================

**Use secrets from [`pass`](https://www.passwordstore.org) in AWS Parameter Store without leaking the secrets into Terraform statefiles.**

This is forked from the [AWS Terraform Provider](https://github.com/terraform-providers/terraform-provider-aws) and released under the same license.

Why is this needed?
-------------------

The AWS Terraform Provider offers a resource for parameters in Parameter Store. But it leaks secrets into your statefiles:

> Note: The unencrypted value of a SecureString will be stored in the raw state as plain-text. Read more about sensitive data in state.
>
> –[terraform.io/docs/providers/aws/r/ssm_parameter.html](https://www.terraform.io/docs/providers/aws/r/ssm_parameter.html)

This Terraform Provider is aimed at working around this limitation in the particular case where you are storing secrets in `pass` and deploying them to Parameter Store. This is common if you want to have a safe, encrypted copy of your secrets (e.g., keys) but also need to have them deployed in AWS to be available for your application.

Usage
-----

This provides two Terraform resources:

  - A data source for checking that a named parameter exists in
    Parameter Store;

  - A resource that inserts values from [`pass`](https://www.passwordstore.org) into Parameter Store.


Requirements
------------

- [Terraform](https://www.terraform.io/downloads.html) 0.10+
- [Go](https://golang.org/doc/install) 1.11 (to build the provider plugin)

Building The Provider
---------------------

```sh
$ make build
```

Using the provider
----------------------
If you're building the provider, follow the instructions to [install it as a plugin.](https://www.terraform.io/docs/plugins/basics.html#installing-a-plugin) After placing it into your plugins directory,  run `terraform init` to initialize it. Documentation about the provider specific configuration options can be found on the [provider's website](https://www.terraform.io/docs/providers/aws/index.html).

Developing the Provider
---------------------------

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (version 1.11+ is *required*). You'll also need to correctly setup a [GOPATH](http://golang.org/doc/code.html#GOPATH), as well as adding `$GOPATH/bin` to your `$PATH`.

```sh
$ make build
...
```

In order to test the provider, you can simply run `make test`.

*Note:* Make sure no `AWS_ACCESS_KEY_ID` or `AWS_SECRET_ACCESS_KEY` variables are set, and there's no `[default]` section in the AWS credentials file `~/.aws/credentials`.

```sh
$ make test
```
