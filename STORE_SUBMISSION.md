# App Store Submission Guide

Thank you for contributing to the PowerLab App Store! To maintain a premium experience for all users, we require all submissions to follow these standards.

## Directory Structure

Each app must live in its own directory under `store/Apps/`:

```text
store/Apps/
└── <your-app-id>/
    └── docker-compose.yml
```

- **App ID:** Must be lowercase, alphanumeric, and use hyphens for spaces (e.g., `uptime-kuma`).

## Docker Compose Requirements

Your `docker-compose.yml` must include a valid `x-powerlab` extension block with the following fields:

| Field | Description | Requirement |
|---|---|---|
| `title.en_us` | Human-readable name of the app. | Required |
| `icon` | URL to a high-resolution transparent PNG/SVG. | Required |
| `tagline.en_us` | A one-line catchy summary. | Required |
| `description.en_us`| A detailed multi-line description. | Required |
| `developer` | The original software creator. | Required |
| `author` | The person who packaged the app for PowerLab. | Required |
| `category` | e.g., Network, Media, Files, Home Automation. | Required |
| `web` | The host port for the "Open UI" button. | Required |
| `main` | The primary service name in the compose file. | Required |

### Example Block

```yaml
x-powerlab:
  title:
    en_us: My Awesome App
  icon: https://raw.githubusercontent.com/.../icon.png
  description:
    en_us: A detailed description of what this app does.
  developer: AwesomeDev
  author: YourName
  category: Utilities
  tagline:
    en_us: Does everything perfectly.
  web: "8080"
  main: app-service
```

## Best Practices

### 1. Icons
- **Format:** Transparent PNG or SVG.
- **Size:** Minimum 256x256 pixels.
- **Visuals:** Should be clean and recognizable against both dark and light backgrounds.

### 2. Networking
- **Port Conflicts:** Don't worry about port conflicts; PowerLab automatically remaps ports if they are already in use. However, specify a sensible default.
- **Network Mode:** Avoid `network_mode: host` unless absolutely necessary (e.g., Home Assistant).

### 3. Storage
- **Persistence:** Always use `/DATA/PowerLabAppData/<app-id>` for bind mounts on the host side. This ensures portability and automatic remapping on different OSs (macOS/Linux). See [ADR-0021](docs/decisions/0021-docker-label-namespace-and-appdata-path.md) for the rationale.
  ```yaml
  volumes:
    - /DATA/PowerLabAppData/my-app/config:/config
  ```

### 4. Resource Limits
- Be a good neighbor. If the app is resource-intensive, consider adding `deploy.resources.limits`.

## Validation

All PRs are automatically validated by our CI suite. To run the validation locally, use:

```bash
go run scripts/validate_store.go
```

The validator checks for:
- YAML syntax errors.
- Missing required `x-powerlab` fields.
- Image existence (optional/warning).
- Icon URL accessibility.
