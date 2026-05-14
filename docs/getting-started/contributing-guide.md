# Getting Started for Developers

Welcome to the PowerLab codebase. If you want to contribute, this guide will help you build a mental model of the system and get your local development environment running.

## Mental Model

PowerLab is an open-source personal cloud OS. It's built on a decoupled architecture where the backend provides APIs and the frontend is a pure, static SPA.

1. **The Backend (Go)**: A microservice architecture composed of 6 distinct services. 
    - **gateway**: The front door. It handles reverse-proxying, TLS (HTTPS), and mDNS.
    - **user-service**: Authentication and user management.
    - **app-management**: Docker Compose lifecycle (the "App Store").
    - **core**: System metrics and hardware monitoring.
    - **local-storage**: Disk management and USB hotplugging.
    - **message-bus**: Pub/sub layer for inter-service communication via SSE/WebSockets.
2. **The Frontend (SvelteKit)**: A static SPA built with Svelte 5 Runes and Tailwind v4. It talks exclusively to the `gateway`, which proxies the requests to the respective microservices.
3. **The Contract (OpenAPI)**: Every microservice exposes its REST API via an `openapi.yaml` file. We use `oapi-codegen` to generate the Go interfaces and validation middleware, ensuring the code always matches the documentation.

## The Local Development Flow

You don't need to manually start 6 microservices and a UI dev server. We have a single script that orchestrates everything.

### Prerequisites

- **Go 1.25+**
- **Node.js 20+**
- **Docker Engine** (running)

### 1. Start the Stack

From the root of the repository, run:

```bash
./dev.sh
```

This script will:
- Check your prerequisites.
- Install `npm` dependencies for the UI if missing.
- Build the 6 Go microservices into the `backend/runtime/bin/` folder.
- Start the Go microservices (with the gateway on port `8765`).
- Start the Vite development server for the UI.

You can stop the stack at any time with `Ctrl+C`.

### 2. View the App

The UI dev server runs at:
```
http://localhost:5173
```
*Note: Vite automatically proxies API requests to the gateway running on port 8765.*

### 3. View the API Docs

PowerLab embeds an interactive API documentation portal powered by Scalar. Once the backend is running, you can view the live docs for all 6 microservices at:
```
http://localhost:8765/docs
```

## Making Changes

### Backend Changes

If you modify Go code, you need to restart the backend. 
- You can restart the whole stack by stopping `dev.sh` and running it again.
- Or, you can run `./dev.sh --no-build` if you only touched frontend code and don't want to wait for the Go compiler.

If you modify an `openapi.yaml` file, you **must** regenerate the Go code before building:
```bash
cd backend/<service>
go generate ./...
```

### Frontend Changes

If you modify code in the `ui/` directory, Vite provides Hot Module Replacement (HMR). You don't need to restart anything; your browser will update instantly.

## Next Steps

- Read the [Architecture Overview](../architecture/README.md) to understand how the services interact.
- Read [CONTRIBUTING.md](https://github.com/neochaotic/powerlab/blob/main/CONTRIBUTING.md) for our coding standards and PR process.
- Browse the [ADRs](../decisions/README.md) to understand why certain technical choices were made.
