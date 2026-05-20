# Smokeping

SmokePing continuously measures network latency to a set of targets and generates beautiful round-robin RRD graphs showing latency trends and packet loss over time. Essential for diagnosing intermittent connectivity issues.

**First steps:** Open `http://<host>:10280` to view the latency graphs. Edit `Targets` in the config to add your own hosts.

**Data:** `/DATA/PowerLabAppData/smokeping/` — RRD (round-robin database) files with historical latency measurements.
