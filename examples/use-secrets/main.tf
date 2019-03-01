terraform {
  backend "local" {
    path = "use-secrets.tfstate"
  }
}

provider "parameterstore" {
  region = "eu-west-1"
}

data "parameterstore_parameter" "foo" {
  parameter_name = "foo"
}

output "foo_arn" {
  value = "${data.parameterstore_parameter.foo.arn}"
}
