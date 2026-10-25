//go:build js

// Package api serves a repository over HTTP; on the phone its desk is empty.
// It has one repository, and no workbooks to import (desk.go).
package api

import "net/http"

func (*Server) desk(*http.ServeMux) {}
