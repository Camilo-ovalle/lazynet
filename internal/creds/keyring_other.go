// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package creds

// Windows (DPAPI / Credential Manager) and macOS (Keychain) backends are
// planned; until then the grace period only applies within one process.
const storeName = "memory (this process only)"

func osStore() Store { return nil }
