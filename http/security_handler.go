// Copyright (c) Bas van Beek 2024.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package http holds a run.Group compatible HTTP Server.
package http

import (
	"net/http"
)

// SecurityHandler holds a middleware to inject HTTP headers to secure the browser.
func SecurityHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("Pragma", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Add("Content-Security-Policy", "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'self';") //nolint:lll // for clarity
		w.Header().Add("X-Frame-Options", "DENY")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Add("Referrer-Policy", "no-referrer")
		w.Header().Add("Feature-Policy", "camera 'none'; microphone 'none'; geolocation 'none'; encrypted-media 'none'; payment 'none'; usb 'none';") //nolint:lll // for clarity
		w.Header().Add("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}
