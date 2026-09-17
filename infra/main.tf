data "aws_vpc" "default" {
  default = true
}

data "aws_ssm_parameter" "al2023_ami" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

# --- Security groups ---------------------------------------------------

resource "aws_security_group" "app" {
  name        = "ssdlc-example-app"
  description = "ssdlc-example app EC2: public HTTP"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "HTTP from anywhere (public app, AD-007)"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "All outbound (docker pull from GHCR, package updates)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "ssdlc-example-app"
  }
}

resource "aws_security_group" "sonar" {
  name        = "ssdlc-example-sonarqube"
  description = "ssdlc-example SonarQube EC2: public 9000"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "SonarQube UI/API from anywhere (GitHub runners have no fixed IP, AD-007)"
    from_port   = 9000
    to_port     = 9000
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "All outbound (docker pull, package updates)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "ssdlc-example-sonarqube"
  }
}

# --- EC2 instances -------------------------------------------------------

resource "aws_instance" "app" {
  ami                    = data.aws_ssm_parameter.al2023_ami.value
  instance_type          = "t3.micro"
  iam_instance_profile   = var.instance_profile_name
  vpc_security_group_ids = [aws_security_group.app.id]
  user_data              = file("${path.module}/user_data/app.sh")

  metadata_options {
    http_tokens = "required"
  }

  root_block_device {
    volume_type = "gp3"
    volume_size = 8
    encrypted   = true
  }

  tags = {
    Name = "ssdlc-example-app"
  }
}

resource "aws_instance" "sonar" {
  ami                    = data.aws_ssm_parameter.al2023_ami.value
  instance_type          = "t3.medium"
  iam_instance_profile   = var.instance_profile_name
  vpc_security_group_ids = [aws_security_group.sonar.id]
  user_data              = file("${path.module}/user_data/sonar.sh")

  metadata_options {
    http_tokens = "required"
  }

  root_block_device {
    volume_type = "gp3"
    volume_size = 30
    encrypted   = true
  }

  tags = {
    Name = "ssdlc-example-sonarqube"
  }
}

# --- Elastic IPs -----------------------------------------------------------

resource "aws_eip" "app" {
  domain   = "vpc"
  instance = aws_instance.app.id

  tags = {
    Name = "ssdlc-example-app"
  }
}

resource "aws_eip" "sonar" {
  domain   = "vpc"
  instance = aws_instance.sonar.id

  tags = {
    Name = "ssdlc-example-sonarqube"
  }
}
