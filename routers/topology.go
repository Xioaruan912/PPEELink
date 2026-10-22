package routers

import (
	"github.com/gin-gonic/gin"
	"ppeelink/api"
)

// NetworkTopology must be registered behind the JWT admin scope for /api/v1/network.
func NetworkTopology(r *gin.Engine) {
	r.GET("/api/v1/network/topology", api.NetworkTopology)
}
