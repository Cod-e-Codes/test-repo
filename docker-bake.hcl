  GNU nano 8.5 docker-bake.hcl                                                                                      
    target "marchat-server" {
      context = "." // The build context (e.g., current directory)
      dockerfile = "./Dockerfile" // Path to your Dockerfile
      tags = ["marchat-server:latest", "marchat-server:v0.1"] // Image tags
      platforms = ["linux/amd64"] // Target platforms for multi-arch builds
      args = { // Build arguments
        VERSION = "0.1"
      }
    }
