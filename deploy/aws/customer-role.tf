# Terraform equivalent of customer-role.yaml (hand this to customers who manage IAM with Terraform).
variable "platform_account_id" { type = string }
variable "external_id" { type = string }

data "aws_iam_policy_document" "trust" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${var.platform_account_id}:root"]
    }
    condition {
      test     = "StringEquals"
      variable = "sts:ExternalId"
      values   = [var.external_id]
    }
  }
}

resource "aws_iam_role" "reliabilix" {
  name               = "ReliabilixReadOnly" # must start with "Reliabilix"
  assume_role_policy = data.aws_iam_policy_document.trust.json
}

resource "aws_iam_role_policy" "cost_explorer" {
  name = "CostExplorerRead"
  role = aws_iam_role.reliabilix.id
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = ["ce:GetCostAndUsage", "ce:GetRightsizingRecommendation"], Resource = "*" }]
  })
}

output "role_arn" { value = aws_iam_role.reliabilix.arn }
