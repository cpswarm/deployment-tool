package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

const endpoint = "http://localhost:8080"

func TestDeploy(t *testing.T) {

	var err error
	var token string

	t.Run("Get a token", func(t *testing.T) {
		token, err = getToken()
		if err != nil {
			t.Fatalf("%s", err)
		}
	})
	t.Log(token)

}

func getToken() (string, error) {
	resp, err := http.Post(endpoint+"/token_sets?name=test&total=1", "none", nil)
	if err != nil {
		return "", fmt.Errorf("error requesting token: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201 but got %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	respMap := make(map[string]interface{})
	err = decoder.Decode(&respMap)
	if err != nil {
		return "", fmt.Errorf("error decoding response: %s", err)
	}

	if _, found := respMap["tokens"]; !found {
		return "", fmt.Errorf("tokens not found in response:\n%s", spew.Sdump(respMap))
	}

	tokens, ok := respMap["tokens"].([]interface{})
	if !ok {
		return "", fmt.Errorf("type assertion not possible for tokens in response:\n%s", spew.Sdump(respMap))
	}

	if len(tokens) != 1 {
		return "", fmt.Errorf("expected 1 token but got %d", len(tokens))
	}

	token, ok := tokens[0].(string)
	if !ok {
		return "", fmt.Errorf("type assertion not possible for token in response:\n%s", spew.Sdump(respMap))
	}

	return token, nil
}
