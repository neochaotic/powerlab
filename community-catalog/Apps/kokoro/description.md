# Kokoro

Kokoro is a local text-to-speech server powered by the Kokoro TTS model. It exposes an OpenAI-compatible `/v1/audio/speech` API, making it a drop-in offline replacement for cloud TTS in apps that support OpenAI's audio endpoint.

**API endpoint:** Kokoro is a backend service. Point any OpenAI TTS-compatible client at `http://<host>:8880`. No browser UI.

**Data:** `/DATA/PowerLabAppData/kokoro/` — TTS model weights and generation cache.
