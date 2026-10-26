package middleware

import (
"net/http"
"strings"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
)

// CORS allows cross-origin requests from an explicit allow-list so
// browser-based clients (a web admin panel, Flutter web build, etc.) can
// call this API directly. Native mobile apps aren't affected by CORS and
// don't need this.
//
// The allow-list comes from the ALLOWED_ORIGINS env var (comma-separated),
// e.g. "https://myshop.com,https://admin.myshop.com". Defaults to common
// local dev server ports if unset - see config.LoadConfig.
//
// SECURITY NOTE: this used to be "Access-Control-Allow-Origin: *", which let
// any website on the internet make authenticated requests using a visitor's
// token if it ever leaked (XSS on an unrelated site, malicious extension,
// etc.). Locking this to known origins closes that off.
func CORS() gin.HandlerFunc {
allowed := parseOrigins(config.AppConfig.AllowedOrigins)

return func(c *gin.Context) {
origin := c.GetHeader("Origin")

if origin != "" && isAllowedOrigin(origin, allowed) {
c.Header("Access-Control-Allow-Origin", origin)
c.Header("Vary", "Origin")
c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
c.Header("Access-Control-Allow-Credentials", "true")
}

if c.Request.Method == http.MethodOptions {
c.AbortWithStatus(http.StatusNoContent)
return
}
c.Next()
}
}

func parseOrigins(raw string) []string {
var out []string
for _, o := range strings.Split(raw, ",") {
o = strings.TrimSpace(o)
if o != "" {
out = append(out, o)
}
}
return out
}

func isAllowedOrigin(origin string, allowed []string) bool {
for _, a := range allowed {
if a == "*" || a == origin {
return true
}
}
return false
}
