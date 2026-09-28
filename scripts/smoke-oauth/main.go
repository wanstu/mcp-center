package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	baseURL := flag.String("base-url", "", "MCP Center base URL")
	username := flag.String("username", "", "admin username")
	password := flag.String("password", "", "admin password")
	mcpRef := flag.String("mcp-ref", "", "MCP id or name")
	flag.Parse()
	if *baseURL == "" || *username == "" || *password == "" || *mcpRef == "" {
		fail("base-url, username, password and mcp-ref are required")
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 45 * time.Second}

	postJSON(client, *baseURL+"/api/auth/login", map[string]any{
		"username": *username,
		"password": *password,
	}, nil)

	var started struct {
		Test struct {
			ID string `json:"id"`
		} `json:"test"`
		AuthorizeURL string `json:"authorize_url"`
	}
	postJSON(client, *baseURL+"/api/oauth/tests", map[string]any{"mcp_ref": *mcpRef}, &started)
	if started.Test.ID == "" || started.AuthorizeURL == "" {
		fail("OAuth self-test did not return test id and authorize URL")
	}

	resp, err := client.Get(started.AuthorizeURL)
	if err != nil {
		fail("authorize GET: " + err.Error())
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "授权 MCP 客户端") {
		fail(fmt.Sprintf("authorize GET = %d", resp.StatusCode))
	}

	u, err := url.Parse(started.AuthorizeURL)
	if err != nil {
		fail("authorize URL parse: " + err.Error())
	}
	form := u.Query()
	form.Set("decision", "approve")
	resp, err = client.PostForm(strings.TrimRight(*baseURL, "/")+"/oauth/authorize", form)
	if err != nil {
		fail("authorize POST/callback: " + err.Error())
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "OAuth 自测通过") {
		fail(fmt.Sprintf("authorize callback = %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var status struct {
			Test struct {
				Status string `json:"status"`
				Error string `json:"error"`
				Steps []struct {
					Key string `json:"key"`
					Status string `json:"status"`
					Detail string `json:"detail"`
				} `json:"steps"`
			} `json:"test"`
		}
		getJSON(client, strings.TrimRight(*baseURL, "/")+"/api/oauth/tests/"+url.PathEscape(started.Test.ID), &status)
		switch status.Test.Status {
		case "passed":
			for _, step := range status.Test.Steps {
				if step.Status != "passed" {
					fail(fmt.Sprintf("OAuth step %s = %s: %s", step.Key, step.Status, step.Detail))
				}
			}
			fmt.Println("OAuth self-test passed.")
			return
		case "failed":
			fail("OAuth self-test failed: " + status.Test.Error)
		}
		time.Sleep(200 * time.Millisecond)
	}
	fail("timed out waiting for OAuth self-test")
}

func postJSON(client *http.Client, endpoint string, value any, out any) {
	data, err := json.Marshal(value)
	if err != nil { fail(err.Error()) }
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil { fail(err.Error()) }
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil { fail(err.Error()) }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail(fmt.Sprintf("POST %s = %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body))))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil { fail(err.Error()) }
	}
}

func getJSON(client *http.Client, endpoint string, out any) {
	resp, err := client.Get(endpoint)
	if err != nil { fail(err.Error()) }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fail(fmt.Sprintf("GET %s = %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body))))
	}
	if err := json.Unmarshal(body, out); err != nil { fail(err.Error()) }
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
