# Ollama (AMD GPU / ROCm)

Ollama is a local LLM inference server for running Llama 3, Mistral, Gemma, Phi-3, and other open models. This variant uses ROCm for AMD GPU acceleration (`/dev/kfd`). Exposes an OpenAI-compatible REST API.

**API endpoint:** Ollama is a backend service. Pull a model via CLI: `docker exec <container> ollama pull llama3`. Then point an LLM UI (Open WebUI, Chatbot UI) at `http://<host>:11434`. Requires an AMD GPU with ROCm support.

**Data:** `/DATA/PowerLabAppData/ollama-amd/` — Downloaded model weights (can be large — tens of GB per model).
