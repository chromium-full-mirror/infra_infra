create {
  platform_re: "linux-amd64|windows-amd64"
  source {
    script {
      name: ["pull_current_nvidia_drivers.py"]
    }
  }
}