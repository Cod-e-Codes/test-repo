# Plugin Registry

This directory is **not used** by the marchat plugin system.

## Remote Registry

The marchat plugin system uses the remote GitHub registry located at:
- **URL**: https://raw.githubusercontent.com/Cod-e-Codes/marchat-plugins/main/registry.json
- **Repository**: https://github.com/Cod-e-Codes/marchat-plugins

## Configuration

The plugin registry URL can be configured using the `MARCHAT_PLUGIN_REGISTRY_URL` environment variable. By default, it points to the GitHub registry.

```bash
# Default (GitHub registry)
export MARCHAT_PLUGIN_REGISTRY_URL="https://raw.githubusercontent.com/Cod-e-Codes/marchat-plugins/main/registry.json"

# Custom registry (optional)
export MARCHAT_PLUGIN_REGISTRY_URL="https://my-registry.com/plugins.json"
```

## Local Development

For local development or testing, you can create a local registry file and set the URL to:
```bash
export MARCHAT_PLUGIN_REGISTRY_URL="file:///path/to/your/local/registry.json"
```

## Plugin Installation

Plugins are installed using the `:plugin install <name>` command in the marchat client.

