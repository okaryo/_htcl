package http1

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func BasicAuthorizationValue(username, password string) (string, error) {
	if strings.ContainsAny(username, "\r\n") {
		return "", fmt.Errorf("username contains line break")
	}
	if strings.ContainsAny(password, "\r\n") {
		return "", fmt.Errorf("password contains line break")
	}

	credentials := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials)), nil
}
