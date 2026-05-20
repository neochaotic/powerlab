# Ollama (CPU)

Ollama is a local LLM inference server for running Llama 3, Mistral, Gemma, Phi-3, and other open models. This variant uses CPU-only inference — slower, but works on any machine. Exposes an OpenAI-compatible REST API.

**API endpoint:** Ollama is a backend service. Pull a model: `docker exec <container> ollama pull llama3`. Then point an LLM UI at `http://<host>:11434`.

**Data:** `/DATA/PowerLabAppData/ollama-cpu/` — Downloaded model weights (can be tens of GB per model).
