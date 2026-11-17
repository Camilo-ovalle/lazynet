// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package lazynet embeds files from the repository root that the program
// needs at run time.
package lazynet

import _ "embed"

// Changelog is CHANGELOG.md, the release notes of every version. The update
// banner and `lazynet changelog` read it (see internal/update).
//
//go:embed CHANGELOG.md
var Changelog string
