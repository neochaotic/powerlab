# Emby_Nvidia

Emby with NVIDIA GPU hardware transcoding enabled. Identical to the standard Emby image but configured to use `runtime: nvidia` for H.264/HEVC hardware acceleration — dramatically reduces CPU load for simultaneous streams.

**First steps:** Open `http://<host>:8096`, complete the setup wizard, then enable hardware transcoding in Admin → Transcoding.

**Data:** `/DATA/PowerLabAppData/emby-nvidia/` — Emby database and transcoding cache. Requires NVIDIA Container Toolkit on the host.
