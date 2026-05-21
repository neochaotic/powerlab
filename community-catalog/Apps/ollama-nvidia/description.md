# Ollama (NVIDIA GPU)

Ollama is a local LLM inference server for running Llama 3, Mistral, Gemma, Phi-3, and other open models. This variant uses CUDA for NVIDIA GPU acceleration. Exposes an OpenAI-compatible REST API.

**API endpoint:** Ollama is a backend service. Pull a model: `docker exec <container> ollama pull llama3`. Then point an LLM UI at `http://<host>:11434`. Requires NVIDIA Container Toolkit on the host.

**Data:** `/DATA/PowerLabAppData/ollama-nvidia/` — Downloaded model weights (can be tens of GB per model).
