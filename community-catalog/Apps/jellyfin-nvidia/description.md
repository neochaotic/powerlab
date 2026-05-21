# Jellyfin_Nvidia

Jellyfin with NVIDIA GPU hardware transcoding. Identical to the standard Jellyfin image but configured with `runtime: nvidia` for H.264/HEVC/AV1 hardware acceleration, reducing CPU load for concurrent streams.

**First steps:** Open `http://<host>:8097`, complete the setup wizard, then enable hardware transcoding under Admin → Playback.

**Data:** `/DATA/PowerLabAppData/jellyfin-nvidia/` — Jellyfin database and transcoding cache. Requires NVIDIA Container Toolkit on the host.
