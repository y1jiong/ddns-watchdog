package server

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// writeJson sends body as a JSON response with the given status code. Every
// response, including a rejection, carries a body so that a client can report why
// the request was not served.
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

func RespGetIPReq(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	if req.Method != http.MethodGet {
		writeJson(w, http.StatusMethodNotAllowed, common.GeneralResp{Message: "method not allowed, use GET"})
		return
	}

	writeJson(w, http.StatusOK, common.GetIPResp{
		IP:      GetClientIP(req),
		Version: Srv.GetLatestVersion(),
	})
}

func RespCenterReq(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	if req.Method != http.MethodPost {
		writeJson(w, http.StatusMethodNotAllowed, common.GeneralResp{Message: "method not allowed, use POST"})
		return
	}

	bodyJson, err := io.ReadAll(req.Body)
	if err != nil {
		log.Println(err)
		writeJson(w, http.StatusBadRequest, common.GeneralResp{Message: "cannot read request body"})
		return
	}

	var body common.CenterReq
	if err = json.Unmarshal(bodyJson, &body); err != nil {
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
