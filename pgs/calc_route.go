package pgs

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/picosh/pico/shared"
	"github.com/picosh/pico/shared/storage"
	"github.com/picosh/send/send/utils"
)

type HttpReply struct {
	Filepath string
	Query    map[string]string
	Status   int
}

func expandRoute(projectName, fp string, status int) []*HttpReply {
	if fp == "" {
		fp = "/"
	}
	mimeType := storage.GetMimeType(fp)
	fname := filepath.Base(fp)
	fdir := filepath.Dir(fp)
	fext := filepath.Ext(fp)
	routes := []*HttpReply{}

	if mimeType != "text/plain" {
		return routes
	}

	if fext == ".txt" {
		return routes
	}

	// we know it's a directory so send the index.html for it
	if strings.HasSuffix(fp, "/") {
		dirRoute := shared.GetAssetFileName(&utils.FileEntry{
			Filepath: filepath.Join(projectName, fp, "index.html"),
		})

		routes = append(
			routes,
			&HttpReply{Filepath: dirRoute, Status: status},
		)
	} else {
		if fname == "." {
			return routes
		}

		// pretty urls where we just append .html to base of fp
		nameRoute := shared.GetAssetFileName(&utils.FileEntry{
			Filepath: filepath.Join(
				projectName,
				fdir,
				fmt.Sprintf("%s.html", fname),
			),
		})

		routes = append(
			routes,
			&HttpReply{Filepath: nameRoute, Status: status},
		)
	}

	return routes
}

func checkIsRedirect(status int) bool {
	return status >= 300 && status <= 399
}

func correlatePlaceholder(orig, pattern string) string {
	origList := strings.Split(orig, "/")
	patternList := strings.Split(pattern, "/")
	for idx, item := range patternList {
		if strings.HasPrefix(item, ":") {
			origList[idx] = item
		}
	}
	finList := []string{}
	if strings.HasPrefix(orig, "/") {
		finList = append(finList, "/")
	}
	finList = append(finList, origList...)
	return filepath.Join(finList...)
}

func genRedirectRoute(actual string, fromStr string, to string) string {
	actualList := strings.Split(actual, "/")
	fromList := strings.Split(fromStr, "/")
	toList := strings.Split(to, "/")

	mapper := map[string]string{}
	for idx, item := range fromList {
		if strings.HasPrefix(item, ":") {
			mapper[item] = actualList[idx]
		}
	}

	fin := []string{"/"}
	for _, item := range toList {
		if mapper[item] != "" {
			fin = append(fin, mapper[item])
		} else {
			fin = append(fin, item)
		}
	}

	return filepath.Join(fin...)
}

func calcRoutes(projectName, fp string, userRedirects []*RedirectRule) []*HttpReply {
	rts := []*HttpReply{}
	// add route as-is without expansion
	if fp != "" && !strings.HasSuffix(fp, "/") {
		defRoute := shared.GetAssetFileName(&utils.FileEntry{
			Filepath: filepath.Join(projectName, fp),
		})
		rts = append(rts, &HttpReply{Filepath: defRoute, Status: http.StatusOK})
	}
	expts := expandRoute(projectName, fp, http.StatusOK)
	rts = append(rts, expts...)

	// user routes
	for _, redirect := range userRedirects {
		// this doesn't make sense so it is forbidden
		if redirect.From == redirect.To {
			continue
		}

		// hack: make suffix `/` optional when matching
		from := filepath.Clean(redirect.From)
		fromMatcher := correlatePlaceholder(fp, from)
		rr := regexp.MustCompile(fromMatcher)
		match := rr.FindStringSubmatch(fp)
		if len(match) > 0 {
			isRedirect := checkIsRedirect(redirect.Status)
			if !isRedirect {
				if !hasProtocol(redirect.To) {
					route := genRedirectRoute(fp, from, redirect.To)
					fmt.Println(route)
					// wipe redirect rules to prevent infinite loops
					// as such we only support a single hop for user defined redirects
					redirectRoutes := calcRoutes(projectName, route, []*RedirectRule{})
					rts = append(rts, redirectRoutes...)
					return rts
				}
			}

			route := genRedirectRoute(fp, from, redirect.To)
			userReply := []*HttpReply{}
			var rule *HttpReply
			if redirect.To != "" {
				rule = &HttpReply{
					Filepath: route,
					Status:   redirect.Status,
					Query:    redirect.Query,
				}
				userReply = append(userReply, rule)
			}

			if redirect.Force {
				rts = userReply
			} else {
				rts = append(rts, userReply...)
			}
			// quit after first match
			break
		}
	}

	// filename without extension mean we might have a directory
	// so add a trailing slash with a 301
	if fp != "" && !strings.HasSuffix(fp, "/") {
		redirectRoute := shared.GetAssetFileName(&utils.FileEntry{
			Filepath: fp + "/",
		})
		rts = append(
			rts,
			&HttpReply{Filepath: redirectRoute, Status: http.StatusMovedPermanently},
		)
	}

	notFound := &HttpReply{
		Filepath: filepath.Join(projectName, "404.html"),
		Status:   http.StatusNotFound,
	}

	rts = append(rts,
		notFound,
	)

	return rts
}
