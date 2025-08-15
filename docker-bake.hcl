  GNU nano 8.5 docker-bake.hcl                                                                                      
    target "marchat-server" {
      context = "." // The build context (e.g., current directory)
      dockerfile = "./Dockerfile" // Path to your Dockerfile
      tags = ["codecodesxyz/marchat:latest", "codecodesxyz/marchat:v0.3.0-beta.6"] // Image tags
      platforms = ["linux/amd64"] // Target platforms for multi-arch builds
      args = { // Build arguments
        VERSION = "v0.3.0-beta.6"
      }
    }
