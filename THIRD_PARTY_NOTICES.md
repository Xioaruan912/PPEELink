# Third-party notices

## Streaming & AI Unlock Check

PPEELink's curated unlock probes, regional lists and response readers in
`node/unlock_catalog.go`, `node/unlock_checks.go` and
`webs/src/components/node-tests/browserUnlock.ts` adapt portions of
[yt1988/unlock-check](https://github.com/yt1988/unlock-check), pinned to
commit `e941ebde9413ee844ecd34e8ea4ed8ccf7ac0a43` (2026-11-05).

PPEELink implements its own node routing, bounded execution, cancellation,
result classification and UI. The original browser redirect-counting engine,
majority-country inference and external status/DNS services are not included.

MIT License

Copyright (c) 2026 yt1988

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## iperf3 static tool distribution

Agent throughput preparation downloads fixed3.22 assets from
[userdocs/iperf3-static](https://github.com/userdocs/iperf3-static/releases/tag/3.22).
The binary is a third-party musl/OpenSSL static distribution; it is not an ESnet
binary release. Per-architecture SHA256 digests are pinned in
`throughputcore/prepare.go` on the `agent` branch. Neither third-party build scripts nor their automatic
latest-OpenSSL selection are run by PPEELink.

Full iperf3, OpenSSL and musl license notices are preserved in
`throughputcore/licenses/` on the `agent` branch and installed alongside the executable as
`/usr/local/lib/subl/iperf3.LICENSE` before activating the downloaded tool.
