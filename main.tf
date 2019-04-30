# terraform configuration file adapted from https://github.com/snsinfu/terraform-lambda-example

provider "aws" {
  region = "us-west-2"
}

# This is required to get the AWS region via ${data.aws_region.current}.
data "aws_region" "current" {
}

# Define a Lambda function.
#
# The handler is the name of the executable for go1.x runtime.
resource "aws_lambda_function" "lumberjack" {
  function_name    = "lumberjack"
  filename         = "lumberjack.zip"
  handler          = "lumberjack"
  source_code_hash = "${base64sha256(file("lumberjack.zip"))}"
  role             = "${aws_iam_role.lumberjack.arn}"
  runtime          = "go1.x"
  memory_size      = 128
  timeout          = 30
}

# A Lambda function may access to other AWS resources such as S3 bucket. So an
# IAM role needs to be defined.
#
# The date 2012-10-17 is just the version of the policy language used here [1].
#
# [1]: https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_version.html

resource "aws_iam_role_policy" "ses_log_policy" {
  role = "${aws_iam_role.lumberjack.id}"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ses:*",
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams",
        "logs:PutLogEvents",
        "logs:GetLogEvents",
        "logs:FilterLogEvents"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_role" "lumberjack" {
  name               = "lumberjack"
  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
POLICY
}

# Allow API gateway to invoke the lumberjack Lambda function.
resource "aws_lambda_permission" "lumberjack" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.lumberjack.arn}"
  principal     = "apigateway.amazonaws.com"
}

# A Lambda function is not a usual public REST API. We need to use AWS API
# Gateway to map a Lambda function to an HTTP endpoint.
resource "aws_api_gateway_resource" "lumberjack" {
  rest_api_id = "${aws_api_gateway_rest_api.lumberjack.id}"
  parent_id   = "${aws_api_gateway_rest_api.lumberjack.root_resource_id}"
  path_part   = "lumberjack"
}

# Binary media types are necessary for contact forms with binary file
# attachments, but imply that the whole Body field of the
# events.APIGatewayProxyRequest structure (including text portions and
# mime headers) is base64 encoded and must be decoded before being
# parsed.
resource "aws_api_gateway_rest_api" "lumberjack" {
  name = "lumberjack"
  binary_media_types = ["multipart/form-data"]
}

#           POST
# Internet -----> API Gateway
resource "aws_api_gateway_method" "lumberjack" {
  rest_api_id   = "${aws_api_gateway_rest_api.lumberjack.id}"
  resource_id   = "${aws_api_gateway_resource.lumberjack.id}"
  http_method   = "POST"
  authorization = "NONE"
}

#              POST
# API Gateway ------> Lambda
# For Lambda the method is always POST and the type is always AWS_PROXY.
#
# The date 2015-03-31 in the URI is just the version of AWS Lambda.
resource "aws_api_gateway_integration" "lumberjack" {
  rest_api_id             = "${aws_api_gateway_rest_api.lumberjack.id}"
  resource_id             = "${aws_api_gateway_resource.lumberjack.id}"
  http_method             = "${aws_api_gateway_method.lumberjack.http_method}"
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = "arn:aws:apigateway:${data.aws_region.current.name}:lambda:path/2015-03-31/functions/${aws_lambda_function.lumberjack.arn}/invocations"
}

# This resource defines the URL of the API Gateway.
resource "aws_api_gateway_deployment" "lumberjack_v1" {
  depends_on = [
    "aws_api_gateway_integration.lumberjack"
  ]
  rest_api_id = "${aws_api_gateway_rest_api.lumberjack.id}"
  stage_name  = "v1"
}

# Set the generated URL as an output. Run `terraform output url` to get this.
output "url" {
  value = "${aws_api_gateway_deployment.lumberjack_v1.invoke_url}${aws_api_gateway_resource.lumberjack.path}"
}
