package interactive

import "net/http"

// setCORS will set "Access-Control-Allow-Origin: '*'" allowing request from
// any origin.
func setCORS(w http.ResponseWriter) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
}
