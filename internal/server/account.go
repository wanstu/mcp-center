package server

import "net/http"

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	token, updated, err := a.account.ChangePassword(user.ID, input.CurrentPassword, input.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}
