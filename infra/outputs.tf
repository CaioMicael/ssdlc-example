output "app_url" {
  description = "Public URL of the app EC2 instance."
  value       = "http://${aws_eip.app.public_ip}"
}

output "app_instance_id" {
  description = "Instance ID of the app EC2, used by deploy.yml for SSM send-command."
  value       = aws_instance.app.id
}

output "sonar_url" {
  description = "Public URL of the SonarQube EC2 instance."
  value       = "http://${aws_eip.sonar.public_ip}:9000"
}
