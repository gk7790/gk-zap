package controller

import "github.com/gk7790/gk-zap/server/visitor"

// All resource managers and controllers
type ResourceController struct {
	// Manage all visitor listeners
	VisitorManager *visitor.Manager
}
