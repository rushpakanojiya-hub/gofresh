package handlers

import "github.com/gin-gonic/gin"

// ctxPhone returns the authenticated phone from the Gin context without
// panicking when the key is missing or not a string (BUG-86).
func ctxPhone(c *gin.Context) string {
    v, _ := c.Get("phone")
    s, _ := v.(string)
    return s
}