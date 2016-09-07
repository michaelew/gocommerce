package api

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/mailer"
	"github.com/rs/cors"
	"github.com/satori/go.uuid"
)

var bearerRegexp = regexp.MustCompile(`^(?:B|b)earer (\S+$)`)

// API is the main REST API
type API struct {
	handler    http.Handler
	db         *gorm.DB
	config     *conf.Configuration
	mailer     *mailer.Mailer
	httpClient *http.Client
	log        *logrus.Entry
}

type JWTClaims struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
	*jwt.StandardClaims
}

func (a *API) withConfig(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	return context.WithValue(ctx, "config", a.config)
}

func (a *API) withToken(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ctx
	}

	matches := bearerRegexp.FindStringSubmatch(authHeader)
	if len(matches) != 2 {
		UnauthorizedError(w, "Bad authentication header")
		return nil
	}

	token, err := jwt.ParseWithClaims(matches[1], &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Header["alg"] != "HS256" {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getConfig(ctx).JWT.Secret), nil
	})
	if err != nil {
		UnauthorizedError(w, fmt.Sprintf("Invalid token: %v", err))
		return nil
	}
	claims := token.Claims.(*JWTClaims)
	if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
		UnauthorizedError(w, fmt.Sprintf("Expired token: %v", err))
		return nil
	}

	return context.WithValue(ctx, "jwt", token)
}

// ListenAndServe starts the REST API
func (a *API) ListenAndServe(hostAndPort string) error {
	return http.ListenAndServe(hostAndPort, a.handler)
}

// NewAPI instantiates a new REST API
func NewAPI(config *conf.Configuration, db *gorm.DB, mailer *mailer.Mailer) *API {
	api := &API{
		config:     config,
		db:         db,
		mailer:     mailer,
		httpClient: &http.Client{},
		log:        logrus.NewEntry(logrus.StandardLogger()),
	}
	mux := kami.New()

	mux.Use("/", api.withConfig)
	mux.Use("/", api.withToken)
	mux.Get("/", api.Index)
	mux.Get("/orders", api.trace(api.OrderList))
	mux.Post("/orders", api.OrderCreate)
	mux.Get("/orders/:id", api.trace(api.OrderView))
	mux.Get("/orders/:order_id/payments", api.PaymentList)
	mux.Post("/orders/:order_id/payments", api.PaymentCreate)
	mux.Get("/vatnumbers/:number", api.VatnumberLookup)

	corsHandler := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	api.handler = corsHandler.Handler(mux)
	return api
}

func (a *API) trace(f func(RequestContext, http.ResponseWriter, *http.Request)) func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		id := uuid.NewV4()
		log := a.log.WithFields(logrus.Fields{
			"request_id": id,
			"method":     r.Method,
			"url":        r.URL.String(),
		})
		log.Debug("request started")
		defer log.Debug("request completed")

		reqContext := &RequestContext{ctx}
		reqContext = reqContext.WithLogger(log).WithConfig(a.config)

		f(*reqContext, w, r)
	}
}
