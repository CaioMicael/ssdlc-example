#!/bin/bash
set -euo pipefail

# Amazon Linux 2023: install Docker and enable it so containers restart on boot.
dnf install -y docker
systemctl enable --now docker

# SSM agent ships preinstalled on the AL2023 SSM AMI; make sure it stays enabled.
systemctl enable --now amazon-ssm-agent

# SonarQube requires higher vm.max_map_count and file descriptor limits than the AMI default.
cat > /etc/sysctl.d/99-sonarqube.conf <<'EOF'
vm.max_map_count=524288
fs.file-max=131072
EOF
sysctl --system

# Idempotent: skip if the container already exists (re-run of user_data / reboot).
if ! docker ps -a --format '{{.Names}}' | grep -q '^sonarqube$'; then
  docker run -d --name sonarqube --restart unless-stopped -p 9000:9000 \
    -v sonarqube_data:/opt/sonarqube/data \
    -v sonarqube_extensions:/opt/sonarqube/extensions \
    -v sonarqube_logs:/opt/sonarqube/logs \
    sonarqube:community
fi
