# Agent release versions

Agent versions are independent of panel builds. The initial numbered release is1.11, defined in agentversion.Current. Each subsequent Agent code release increments the second integer:1.12…1.19,1.20…1.29,1.30. Do not interpret the version as a decimal or reset the second part after9. Use agentversion.Next to calculate the next number.

When changing Agent runtime code after this initial release, bump agentversion.Current once for that release. Panel-only UI changes leave it unchanged. Keep binary SHA256 verification for downloads and explicit update acknowledgements; do not use the panel executable hash as the advertised Agent release version or to decide routine update availability.

The monitoring page toolbar opens Agent management. Release status, installed versions, update checks and upgrade/cancel/retry controls live there. Server monitoring cards do not display binary hashes or upgrade status.
