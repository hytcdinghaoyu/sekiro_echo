package middleware

import (
	"log"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"

	"fmt"
	"net/http"
	"reflect"
	"strings"
)

type (
	// JWTConfig defines the config for JWT middleware.
	JWTConfig struct {
		// Signing key to validate token.
		// Required.
		SigningKey interface{}

		// Signing method, used to check token signing method.
		// Optional. Default value HS256.
		SigningMethod string

		// Context key to store user information from the token into context.
		// Optional. Default value "user".
		ContextKey string

		// Claims are extendable claims data defining token content.
		// Optional. Default value jwt.MapClaims
		Claims jwt.Claims

		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		// - "cookie:<name>"
		TokenLookup string

		// AuthScheme to be used in the Authorization header.
		// Optional. Default value "Bearer".
		AuthScheme string

		keyFunc jwt.Keyfunc
	}

	jwtExtractor func(*gin.Context) string
)

// Algorithms
const (
	AlgorithmHS256 = "HS256"
)

var (
	// DefaultJWTConfig is the default JWT auth middleware config.
	DefaultJWTConfig = JWTConfig{
		SigningMethod: AlgorithmHS256,
		ContextKey:    "user",
		TokenLookup:   "header:" + "Authorization",
		AuthScheme:    "Bearer",
		Claims:        jwt.MapClaims{},
	}
)

// JWT returns a JSON Web Token (JWT) auth middleware.
//
// For valid token, it sets the user in context and calls next handler.
// For invalid token, it returns "401 - Unauthorized" error.
// For missing token, it returns "400 - Bad Request" error.
//
// See: https://jwt.io/introduction
// See `JWTConfig.TokenLookup`
func JWT(key interface{}) gin.HandlerFunc {
	c := DefaultJWTConfig
	c.SigningKey = key
	return JWTWithConfig(c)
}

// JWTWithConfig returns a JWT auth middleware with config.
// See: `JWT()`.
func JWTWithConfig(config JWTConfig) gin.HandlerFunc {

	if config.SigningKey == nil {
		panic("echo: jwt middleware requires signing key")
	}
	if config.SigningMethod == "" {
		config.SigningMethod = DefaultJWTConfig.SigningMethod
	}
	if config.ContextKey == "" {
		config.ContextKey = DefaultJWTConfig.ContextKey
	}
	if config.Claims == nil {
		config.Claims = DefaultJWTConfig.Claims
	}
	if config.TokenLookup == "" {
		config.TokenLookup = DefaultJWTConfig.TokenLookup
	}
	if config.AuthScheme == "" {
		config.AuthScheme = DefaultJWTConfig.AuthScheme
	}
	config.keyFunc = func(t *jwt.Token) (interface{}, error) {
		// Check the signing method
		if t.Method.Alg() != config.SigningMethod {
			return nil, fmt.Errorf("unexpected jwt signing method=%v", t.Header["alg"])
		}
		return config.SigningKey, nil
	}

	// Initialize
	parts := strings.Split(config.TokenLookup, ":")
	extractor := jwtFromHeader(parts[1], config.AuthScheme)
	switch parts[0] {
	case "query":
		extractor = jwtFromQuery(parts[1])
	case "cookie":
		extractor = jwtFromCookie(parts[1])
	}

	return func(c *gin.Context) {

		auth := extractor(c)
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "missing or malformed jwt"})
			return
		}

		token := new(jwt.Token)
		var err error
		if _, ok := config.Claims.(jwt.MapClaims); ok {
			token, err = jwt.Parse(auth, config.keyFunc)
		} else {
			t := reflect.ValueOf(config.Claims).Type().Elem()
			claims := reflect.New(t).Interface().(jwt.Claims)
			token, err = jwt.ParseWithClaims(auth, claims, config.keyFunc)
		}
		if err == nil && token.Valid {
			// Store user information from token into context.
			c.Set(config.ContextKey, token)
			c.Next()
		}

		log.Println(err)
		log.Println(token.Valid)

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid or expired jwt"})

	}
}

// jwtFromHeader returns a `jwtExtractor` that extracts token from the request header.
func jwtFromHeader(header string, authScheme string) jwtExtractor {
	return func(c *gin.Context) string {
		auth := c.GetHeader(header)
		l := len(authScheme)
		if len(auth) > l+1 && auth[:l] == authScheme {
			return auth[l+1:]
		}
		return ""
	}
}

// jwtFromQuery returns a `jwtExtractor` that extracts token from the query string.
func jwtFromQuery(param string) jwtExtractor {
	return func(c *gin.Context) string {
		token := c.Query(param)
		if token == "" {
			return ""
		}
		return token
	}
}

// jwtFromCookie returns a `jwtExtractor` that extracts token from the named cookie.
func jwtFromCookie(name string) jwtExtractor {
	return func(c *gin.Context) string {
		cookie, err := c.Cookie(name)
		if err != nil {
			return ""
		}
		return cookie
	}
}