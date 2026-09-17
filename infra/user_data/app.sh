#!/bin/bash
set -euo pipefail

# Amazon Linux 2023: install Docker and enable it so containers restart on boot.
dnf install -y docker
systemctl enable --now docker

# SSM agent ships preinstalled on the AL2023 SSM AMI; make sure it stays enabled.
systemctl enable --now amazon-ssm-agent
