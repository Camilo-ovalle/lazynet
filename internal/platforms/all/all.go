// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package all registers every supported platform. It is the only list of
// platforms in lazynet: to add one, create its package under
// internal/platforms and import it here (see AGENTS.md, "Adding a platform").
package all

import (
	_ "github.com/Camilo-ovalle/lazynet/internal/platforms/cisco/asa"
	_ "github.com/Camilo-ovalle/lazynet/internal/platforms/cisco/catalyst"
	_ "github.com/Camilo-ovalle/lazynet/internal/platforms/cisco/nexus"
	_ "github.com/Camilo-ovalle/lazynet/internal/platforms/paloalto"
)
