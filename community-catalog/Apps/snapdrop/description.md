# Snapdrop

Snapdrop enables AirDrop-like file sharing between devices on the same local network — no accounts, no apps, just open the page on both devices and transfer. Works over WebRTC with a WebSocket signaling server.

**First steps:** Open `http://<host>:443` on two devices on the same network — they discover each other automatically. Click a device icon to send files.

**Data:** `/DATA/PowerLabAppData/snapdrop/` — Stateless signaling server — files transfer peer-to-peer and are not stored.
