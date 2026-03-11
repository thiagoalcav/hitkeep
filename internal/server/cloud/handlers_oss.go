//go:build !billing

package cloud

import (
	"net/http"

	"hitkeep/internal/server/shared"
)

func Register(_ *http.ServeMux, _ *shared.Context) {}
