# Third-party notices

The application's own source is an unreleased internal development candidate; no public distribution license has been chosen. Third-party terms below remain in force. No commercial EDA, PDK, credentials or company project data is bundled.

| Component | Pinned version | License / included notice | Use |
| --- | --- | --- | --- |
| Go runtime/standard library | 1.23.12 | BSD-3-Clause, licenses/Go-LICENSE | Static Linux executable |
| modelcontextprotocol/go-sdk | v1.0.0 | MIT, licenses/mcp-go-sdk-LICENSE | Official MCP protocol server/client |
| google/jsonschema-go | v0.3.0 | MIT, licenses/jsonschema-go-LICENSE | MCP schema validation |
| yosida95/uritemplate/v3 | v3.0.2 | BSD-2-Clause, licenses/uritemplate-LICENSE | MCP URI templates |
| ngspice official public RC example | commit 032b1c32c4dbad45ff132bcfac1dbecadbd8abb0 | Upstream COPYING retained with example | Demo/tests only; topology input, not bundled simulator |

Sources: https://github.com/modelcontextprotocol/go-sdk/tree/v1.0.0 ; https://github.com/google/jsonschema-go/tree/v0.3.0 ; https://github.com/yosida95/uritemplate/tree/v3.0.2 ; https://go.dev/dl/ ; https://github.com/ngspice/ngspice .

Exact module checksums: go.sum. Toolchain download SHA-256 and upstream input/output hashes: docs/validation/toolchain.json and docs/validation/reference-rc.json. Dependency inspection is not a comprehensive security audit. No third-party UI assets or external fonts are used.

## Native desktop (0.7.0-rc1)

The native client requires the user's system Python and Tcl/Tk (CentOS 6.9 validation: Python 2.6.6 and Tk 8.5.7). These runtimes are not bundled in this archive. Python is distributed under its Python Software Foundation terms; Tcl/Tk uses its BSD-style license. Refer to the system packages and https://github.com/tcltk/tk/blob/main/license.terms. No Tauri, Electron, WebKit or Qt code is bundled in the desktop client.

Validation-only, not product dependencies: official Qwen3-1.7B GGUF weights (Apache-2.0), llama.cpp (MIT), Icarus Verilog 13.0 (GPL-2.0+). They remain in the separate validation environment and are not shipped in the production archive. Source, revision/hash and limitations are recorded in REFERENCES.md and docs/NATIVE_DESKTOP.md. A product public-distribution license has not been selected; these notices do not grant rights to proprietary EDA tools or PDKs.
