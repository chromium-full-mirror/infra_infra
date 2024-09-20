create {
  platform_re: "windows-amd64"
  source {
    script {
      name: ["pull_current_nvidia_drivers.py"]
    }
  }
}