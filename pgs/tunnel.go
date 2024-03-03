package pgs

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/ssh"
	"github.com/picosh/pico/db"
	"github.com/picosh/pico/plus"
	"github.com/picosh/pico/shared"
)

func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "You do not have access to this site", http.StatusUnauthorized)
}

func allowPerm(proj *db.Project) bool {
	return true
}

type PicoApi struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"username"`
	PublicKey string `json:"pubkey"`
}

type CtxHttpBridge = func(ssh.Context) http.Handler

func createHttpHandler(httpCtx *shared.HttpCtx) CtxHttpBridge {
	return func(ctx ssh.Context) http.Handler {
		subdomain := ctx.User()
		dbh := httpCtx.Dbpool
		logger := httpCtx.Cfg.Logger
		log := logger.With(
			"subdomain", subdomain,
		)

		pubkey, err := getPublicKeyCtx(ctx)
		if err != nil {
			log.Error(err.Error(), "subdomain", subdomain)
			return http.HandlerFunc(unauthorizedHandler)
		}
		pubkeyStr, err := shared.KeyForKeyText(pubkey)
		if err != nil {
			log.Error(err.Error())
			return http.HandlerFunc(unauthorizedHandler)
		}
		log = log.With(
			"pubkey", pubkeyStr,
		)

		props, err := getProjectFromSubdomain(subdomain)
		if err != nil {
			log.Error(err.Error())
			return http.HandlerFunc(unauthorizedHandler)
		}

		owner, err := dbh.FindUserForName(props.Username)
		if err != nil {
			log.Error(err.Error())
			return http.HandlerFunc(unauthorizedHandler)
		}
		log = log.With(
			"owner", owner.Name,
		)

		project, err := dbh.FindProjectByName(owner.ID, props.ProjectName)
		if err != nil {
			log.Error(err.Error())
			return http.HandlerFunc(unauthorizedHandler)
		}

		requester, _ := dbh.FindUserForKey("", pubkeyStr)
		if requester != nil {
			log = logger.With(
				"requester", requester.Name,
			)
		}

		if !HasProjectAccess(project, owner, requester, pubkey) {
			log.Error("no access")
			return http.HandlerFunc(unauthorizedHandler)
		}

		log.Info("user has access to site")

		routes := []shared.Route{
			// special API endpoint for tunnel users accessing site
			shared.NewRoute("GET", "/pico", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				pico := &PicoApi{
					UserID:    "",
					UserName:  "",
					PublicKey: pubkeyStr,
				}
				if requester != nil {
					pico.UserID = requester.ID
					pico.UserName = requester.Name
				}
				err := json.NewEncoder(w).Encode(pico)
				if err != nil {
					log.Error(err.Error())
				}
			}),
		}

		if subdomain == "hey-plus" || subdomain == "erock-plus" {
			rts := plus.CreateRoutes(httpCtx, ctx)
			routes = append(routes, rts...)
		}

		subdomainRoutes := createSubdomainRoutes(allowPerm)
		routes = append(routes, subdomainRoutes...)
		finctx := httpCtx.CreateCtx(ctx, subdomain)
		httpHandler := shared.CreateServeBasic(routes, finctx)
		httpRouter := http.HandlerFunc(httpHandler)
		return httpRouter
	}
}
