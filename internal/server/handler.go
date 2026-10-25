package server

import (
	"ddns-watchdog/internal/common"
	"encoding/json/v2"
	"io"
	"log"
	"net/http"
	"strings"
)

// NewHandler returns the HTTP handler for the configured routes. The HTTP method
// is part of each ServeMux pattern: the matching handler runs for a supported
// method, and any other method on a configured route gets 405 with an Allow
// header.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+exactRoute(Srv.Route.GetIP), RespGetIPReq)
	if Srv.CenterService {
		mux.HandleFunc("POST "+exactRoute(Srv.Route.Center), RespCenterReq)
	}
	return mux
}

// exactRoute returns an exact ServeMux pattern for route. A trailing slash gets
// the end-of-path wildcard, so the pattern matches only that path and sibling
// routes such as /center keep their own patterns.
func exactRoute(route string) string {
	if strings.HasSuffix(route, "/") {
		return route + "{$}"
	}
	return route
}

// writeJson sends body as a JSON response with the given status code. Callers
// pass a message for a rejection so that a client can report why the request was
// not served.
func writeJson(w http.ResponseWriter, status int, body any) {
	respJson, err := json.Marshal(body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err = w.Write(respJson); err != nil {
		log.Println(err)
	}
}

// RespGetIPReq answers the IP echo endpoint. NewHandler routes GET and HEAD here.
func RespGetIPReq(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	writeJson(w, http.StatusOK, common.GetIPResp{
		IP:      GetClientIP(req),
		Version: Srv.GetLatestVersion(),
	})
}

// RespCenterReq answers the center endpoint. NewHandler routes only POST here.
func RespCenterReq(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	bodyJson, err := io.ReadAll(req.Body)
	if err != nil {
		log.Println(err)
		writeJson(w, http.StatusBadRequest, common.GeneralResp{Message: "cannot read request body"})
		return
	}

	var body common.CenterReq
	if err = json.Unmarshal(bodyJson, &body, json.MatchCaseInsensitiveNames(true)); err != nil {
		log.Println(err)
		writeJson(w, http.StatusBadRequest, common.GeneralResp{Message: "invalid request body"})
		return
	}

	if len(body.Token) > 127 {
		writeJson(w, http.StatusForbidden, common.GeneralResp{Message: "invalid token"})
		return
	}

	instance, ok := getWhitelistEntry(body.Token)
	if !ok {
		writeJson(w, http.StatusForbidden, common.GeneralResp{Message: "invalid token"})
		return
	}
	if !instance.Enable {
		writeJson(w, http.StatusForbidden, common.GeneralResp{Message: "token is disabled"})
		return
	}

	httpStatus, respBody := doVirtualClient(body, instance)
	writeJson(w, httpStatus, respBody)

	if httpStatus == http.StatusOK {
		log.Printf("%v(%v) access successfully.\n", instance.Description, GetClientIP(req))
	}
}
