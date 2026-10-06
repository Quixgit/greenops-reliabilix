# Terraform equivalent of customer-role.yaml (hand this to customers who manage IAM with Terraform).
variable "platform_account_id" { type = string }
variable "external_id" { type = string }

# Optional: location of the customer's FOCUS data export (see docs/setup). Empty = Cost Explorer only.
variable "export_bucket_name" {
  type    = string
  default = ""
}
variable "export_prefix" {
  type    = string
  default = "" # no leading or trailing slash
}

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

# GetObject only, and only below the export's own prefix (no ListBucket, no other objects).
resource "aws_iam_role_policy" "focus_export" {
  count = var.export_bucket_name == "" ? 0 : 1
  name  = "ReadFocusExport"
  role  = aws_iam_role.reliabilix.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "s3:GetObject"
      Resource = var.export_prefix == "" ? "arn:aws:s3:::${var.export_bucket_name}/*" : "arn:aws:s3:::${var.export_bucket_name}/${var.export_prefix}/*"
    }]
  })
}

output "role_arn" { value = aws_iam_role.reliabilix.arn }
