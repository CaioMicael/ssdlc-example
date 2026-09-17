variable "aws_region" {
  description = "AWS region to deploy into."
  type        = string
  default     = "us-east-1"
}

variable "instance_profile_name" {
  description = "Existing IAM instance profile to attach to both EC2 instances (AWS Academy Learner Lab cannot create IAM roles, see AD-005)."
  type        = string
  default     = "LabInstanceProfile"
}
