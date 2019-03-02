terraform {
  backend "local" {
    path = "provision-secrets.tfstate"
  }
}

provider "parameterstore" {
  region = "eu-west-1"
}

resource "parameterstore_parameter_from_pass" "foo" {
  parameter_name = "foo"
  pass_key       = "secrets/foo"
}

output "foo_arn" {
  value = "${parameterstore_parameter_from_pass.foo.arn}"
}
